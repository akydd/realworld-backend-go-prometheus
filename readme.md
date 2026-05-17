# RealWorld Backend — Go

![CI](https://github.com/akydd/realworld-backend-go/actions/workflows/docker-publish.yml/badge.svg)

A [RealWorld](https://github.com/gothinkster/realworld) spec-compliant backend API for a social blogging platform (think Medium.com). Users can register, publish articles, follow each other, comment, and favorite posts.

**Stack:** Go · PostgreSQL · Docker · AWS ECS Fargate · RDS · ALB · Terraform · GitHub Actions

## Key Design Decisions

**Hexagonal Architecture (Ports & Adapters)** — business logic in `internal/domain/` has zero framework dependencies. The HTTP layer and PostgreSQL adapter are fully interchangeable without touching domain code. This makes the codebase easy to test, extend, and reason about.

**AWS ECS Fargate over EC2** — no servers to manage or patch. Tasks run across two private subnets (one per AZ) behind an ALB for high availability and zero-downtime rolling deploys. Application Auto Scaling adjusts the task count between 2 and 4 based on CPU utilization, keeping costs low under normal load while handling traffic spikes automatically.

**Keyless CI/CD via OIDC** — GitHub Actions assumes an AWS IAM role via OpenID Connect rather than using static credentials. No long-lived AWS access keys exist anywhere in the pipeline.

**Separate task execution role and task role** — the execution role has the minimum permissions needed to start a container (pull from ECR, write logs, read secrets). The task role holds only the permissions the running application needs. Compromise of one does not imply compromise of the other.

**Secrets Manager over environment variables** — `DB_PASSWORD` and `JWT_SECRET` are stored in AWS Secrets Manager and injected at container startup. They are never committed to source control or stored in CI.

**Observability via CloudWatch** — ECS CPU/memory, RDS CPU/connections, and ALB 5xx error rate are monitored with CloudWatch alarms. Breaches trigger SNS email notifications, enabling rapid response to availability and performance issues.

## Architecture

The project uses **Hexagonal Architecture** (Ports & Adapters):

- **Domain layer** (`internal/domain/`) — pure Go business logic with no framework dependencies. Each resource (user, profile, article, comment, tag) has its own controller and repository interface.
- **Inbound adapter** (`internal/adapters/in/webserver/`) — Gorilla Mux HTTP server. Handlers decode requests, call domain services, and encode responses. Authentication is handled by JWT middleware.
- **Outbound adapter** (`internal/adapters/out/db/`) — PostgreSQL persistence via `sqlx`. Goose migrations run automatically on startup.

See [arch.md](arch.md) for a full description of every layer, route, schema, and design decision.

## CI/CD

Every push to `main` (and any `v*` tag) runs the following GitHub Actions pipeline. It can also be triggered manually via the **Run workflow** button in the Actions tab.

### Pipeline stages

1. **Test** — checks out the [gothinkster/realworld](https://github.com/gothinkster/realworld) spec repo, installs Hurl, and runs the full integration test suite (`make int-tests`). The build and deploy stages are blocked until this passes.
2. **Build and push** — builds the Docker image and pushes it to Amazon ECR, tagged with the branch name, semver (on tagged releases), and `latest` (on `main`).
3. **Deploy** — triggers a rolling deployment on ECS Fargate by forcing a new deployment of the `realworld-service`. Only runs on pushes to `main`, not on tag pushes. ECS pulls the new `latest` image, starts new tasks, waits for them to pass the ALB health check at `GET /api/healthcheck`, then drains the old tasks.

### Infrastructure

The app runs on AWS in `ca-west-1` using the following services:

- **ECS Fargate** — runs the containerised Go app across two private subnets (one per AZ) for high availability; Application Auto Scaling scales tasks between 2 and 4 based on CPU utilization
- **Application Load Balancer** — receives inbound HTTP traffic on port 80 and forwards to Fargate tasks on port 8090
- **RDS PostgreSQL 17** — database in private subnets, only reachable from ECS tasks
- **ECR** — stores Docker images pushed by the CI pipeline
- **Secrets Manager** — holds `DB_PASSWORD` and `JWT_SECRET`, injected into containers at startup
- **CloudWatch Logs** — container stdout/stderr streamed to `/ecs/realworld` (30 day retention)
- **CloudWatch Alarms + SNS** — email alerts for ECS CPU/memory, RDS CPU/connections, and ALB 5xx error rate

All infrastructure is defined in Terraform under `terraform/`.

### Required secrets

| Secret | Description |
|---|---|
| `AWS_ROLE_ARN` | ARN of the IAM role assumed via OIDC for ECR push and ECS deploy access |

## How it was developed

Features were written as plain-English specification files (e.g. `features/9-create-article.md`). Each feature was implemented with the assistance of **Claude Code**, an AI coding tool. The workflow for each feature was:

1. Write a feature specification describing the required behaviour.
2. Review and guide Claude Code's implementation plan in `features/plans/`.
3. Review the implementation across all required layers.
4. Verify `make lint` reported no issues and `make int-tests` passed all integration tests.
5. Review updates to `arch.md` to keep the architecture document current.

The infrastructure was designed and debugged collaboratively with Claude Code — including VPC layout, IAM policy scoping, ECS service configuration, and resolving deployment issues.

## gRPC API

The server exposes a native gRPC API on port **8099** alongside the existing HTTP API. The gRPC service definitions live in `api/proto/` and the generated Go code is in `api/proto/gen/pb/`. To regenerate after editing a `.proto` file:

```bash
make proto
```

**Why run HTTP and gRPC as separate servers rather than using grpc-gateway?**

[grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway) can translate HTTP/JSON requests into gRPC calls, which sounds appealing — one set of handlers serving both protocols. In practice, making the HTTP path spec-compliant with the RealWorld API spec required too many compromises:

- **Error body shape** — the spec requires `{"errors": {"field": ["message"]}}`. grpc-gateway produces its own JSON error envelope; matching the spec would require a custom error handler rewriting every error response.
- **Status code mismatches** — the spec requires HTTP 422 for validation errors and 409 for duplicates. gRPC's `codes.InvalidArgument` maps to HTTP 400, not 422, with no standard override.
- **Null field semantics** — `PUT /api/user` distinguishes `bio: null` (clear the field) from `bio` absent (leave unchanged). proto3 cannot represent this distinction, so the grpc-gateway HTTP path would silently drop the "clear" behaviour.

Running both servers independently avoids all of this. The existing HTTP server is already fully spec-compliant and integration-tested; the gRPC server provides a typed interface for native gRPC clients. Both delegate to the same domain layer, so there is no business logic duplication.

**Authentication**

Authenticated RPCs expect an `authorization` metadata key with value `Token <jwt>`. Methods that require authentication (`GetUser`, `UpdateUser`, `FollowUser`, `UnfollowUser`, `CreateArticle`, `UpdateArticle`, `FavoriteArticle`, `UnfavoriteArticle`, `DeleteArticle`, `FeedArticles`, `CreateComment`, `DeleteComment`) return `UNAUTHENTICATED` if the token is absent or invalid. Methods with optional auth (`GetProfile`, `GetArticleBySlug`, `ListArticles`, `GetComments`) proceed unauthenticated if no token is supplied. `RegisterUser`, `LoginUser`, and `GetTags` require no token.

**Proto3 limitations vs the HTTP API**

- **`UpdateUser` — bio and image use a `NullableString` wrapper.** `optional string` cannot represent the three states needed (absent = leave unchanged, null = clear, value = set). Both fields use `optional NullableString` instead: omit the field to leave it unchanged, send `bio: {}` to clear it to null, or send `bio: { value: "hello" }` to set a value.
- **`UpdateArticle` — tag list uses a `TagListValue` wrapper.** `repeated string` cannot distinguish absent from empty. The field uses `optional TagListValue` instead: omit to leave tags unchanged, send `tag_list: {}` to clear them, or send `tag_list: { tags: ["go"] }` to replace them.

## Running the app

**Prerequisites:** Docker, Go 1.21+

```bash
make start
```

This will:
1. Start the production PostgreSQL database via Docker Compose
2. Wait until the database is ready
3. Build the server binary
4. Start the server on port **8090**

Stop the server with Ctrl+C. To also stop the database:

```bash
docker compose down
```

## Running the integration tests

**Prerequisites:** Docker, Go 1.21+, [Hurl](https://hurl.dev), and the [realworld](https://github.com/gothinkster/realworld) repo checked out as a sibling directory (`../realworld`).

```bash
make int-tests
```

This will:
1. Start a dedicated test database on port 8096
2. Build and start the server against the test environment (port 8097)
3. Run the full RealWorld Hurl API test suite
4. Tear down the server and test database

## Running the linter

```bash
make lint
```
