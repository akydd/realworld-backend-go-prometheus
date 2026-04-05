# RealWorld Backend — Go

A [RealWorld](https://github.com/gothinkster/realworld) spec-compliant backend API for a social blogging platform (think Medium.com). Users can register, publish articles, follow each other, comment, and favorite posts.

## Architecture

The project uses **Hexagonal Architecture** (Ports & Adapters):

- **Domain layer** (`internal/domain/`) — pure Go business logic with no framework dependencies. Each resource (user, profile, article, comment, tag) has its own controller and repository interface.
- **Inbound adapter** (`internal/adapters/in/webserver/`) — Gorilla Mux HTTP server. Handlers decode requests, call domain services, and encode responses. Authentication is handled by JWT middleware.
- **Outbound adapter** (`internal/adapters/out/db/`) — PostgreSQL persistence via `sqlx`. Goose migrations run automatically on startup.

See [arch.md](arch.md) for a full description of every layer, route, schema, and design decision.

## How it was developed

Features were written as plain-English specification files (e.g. `features/9-create-article.md`). Each feature was handed to **Claude Code**, which:

1. Read the feature file and the current architecture document.
2. Wrote an implementation plan to `features/plans/`.
3. Implemented the plan across all required layers.
4. Iterated until `make lint` reported no issues and `make int-tests` passed all integration tests.
5. Updated `arch.md` to reflect the changes.

The integration test suite is the [official RealWorld Hurl test suite](https://github.com/gothinkster/realworld), run against a live server and test database on every feature.

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

## CI/CD

Every push to `main` (and any `v*` tag) runs the following GitHub Actions pipeline. It can also be triggered manually via the **Run workflow** button in the Actions tab.

### Pipeline stages

1. **Test** — checks out the [gothinkster/realworld](https://github.com/gothinkster/realworld) spec repo, installs Hurl, and runs the full integration test suite (`make int-tests`). The build and deploy stages are blocked until this passes.
2. **Build and push** — builds the Docker image and pushes it to Amazon ECR, tagged with the branch name, semver (on tagged releases), and `latest` (on `main`).
3. **Deploy** — triggers a rolling deployment on ECS Fargate by forcing a new deployment of the `realworld-service`. Only runs on pushes to `main`, not on tag pushes. ECS pulls the new `latest` image, starts new tasks, waits for them to pass the ALB health check at `GET /api/healthcheck`, then drains the old tasks.

### Infrastructure

The app runs on AWS in `ca-west-1` using the following services:

- **ECS Fargate** — runs the containerised Go app across two private subnets (one per AZ) for high availability
- **Application Load Balancer** — receives inbound HTTP traffic on port 80 and forwards to Fargate tasks on port 8090
- **RDS PostgreSQL 17** — database in private subnets, only reachable from ECS tasks
- **ECR** — stores Docker images pushed by the CI pipeline
- **Secrets Manager** — holds `DB_PASSWORD` and `JWT_SECRET`, injected into containers at startup
- **CloudWatch Logs** — container stdout/stderr streamed to `/ecs/realworld` (30 day retention)

All infrastructure is defined in Terraform under `terraform/`.

### Required secrets

| Secret | Description |
|---|---|
| `AWS_ROLE_ARN` | ARN of the IAM role assumed via OIDC for ECR push and ECS deploy access |
