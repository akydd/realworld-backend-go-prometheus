# RealWorld Backend — Go

![CI](https://github.com/akydd/realworld-backend-go/actions/workflows/docker-publish.yml/badge.svg)

A [RealWorld](https://github.com/gothinkster/realworld) spec-compliant backend API for a social blogging platform (think Medium.com), built primarily as a demonstration of **production-style observability** with Prometheus and Grafana.

**Stack:** Go · gRPC · PostgreSQL · Prometheus · Grafana · Docker

---

## Observability

The project implements the **RED method** (Rate, Errors, Duration) for the Go API and full PostgreSQL monitoring, all running in Docker Compose with zero external dependencies.

```
┌──────────┐   scrapes /metrics   ┌────────────┐   queries   ┌─────────┐
│  Go app  │ ──────────────────▶  │ Prometheus │ ──────────▶ │ Grafana │
│  :8090   │                      │   :9090    │             │  :3000  │
└──────────┘                      └────────────┘             └─────────┘
                                        ▲
┌──────────────────┐   scrapes :9187    │
│ postgres_exporter│ ───────────────────┘
└──────────────────┘
        ▲
┌──────────────┐
│  PostgreSQL  │
│    :8095     │
└──────────────┘
```

### Application instrumentation

A single middleware wraps the entire router and records every HTTP request into one `HistogramVec`:

```go
var httpDurationCollector = prometheus.NewHistogramVec(prometheus.HistogramOpts{
    Name: "http_request_duration_ms",
}, []string{"path", "method", "status"})
```

The `path` label is the **route template** (`/api/articles/{slug}`), not the raw URL. Without this, every unique article slug would create its own time series — a cardinality explosion that would make Prometheus unusable at scale. The template is extracted via Gorilla Mux:

```go
func routePattern(r *http.Request) string {
    route := mux.CurrentRoute(r)
    if route == nil {
        return r.URL.Path
    }
    tmpl, err := route.GetPathTemplate()
    if err != nil {
        return r.URL.Path
    }
    return tmpl
}
```

A custom registry ensures `/metrics` only exposes what is explicitly registered — no surprise metrics from third-party libraries.

### RED dashboard queries

One metric gives all three RED signals:

| Signal | PromQL |
|--------|--------|
| **Rate** — requests per second | `sum(rate(http_request_duration_ms_count[5m]))` |
| **Errors** — 4xx/5xx rate by code | `sum by (status) (rate(http_request_duration_ms_count{status=~"4..\|5.."}[5m]))` |
| **Duration** — p99 latency (ms) | `histogram_quantile(0.99, sum by (le) (rate(http_request_duration_ms_bucket[5m])))` |

### PostgreSQL monitoring

`postgres_exporter` is wired into the same Compose network and scraped by Prometheus. A community dashboard from grafana.com/grafana/dashboards gives instant visibility into connections, transaction rates, cache hit ratios, and lock waits with no additional configuration.

### Load testing

A companion project ([realworld-load-test](https://github.com/akydd/realworld-load-test)) uses **k6** to drive realistic traffic against the API — anonymous readers, authenticated users, content creators, a "hot article" with thousands of comments, and intentional 4xx error traffic at production-comparable rates (~4% of total requests). It seeds the database with 1,000 users, 10,000 articles, and realistic follow/favorite graphs before running.

---

## Architecture

The project uses **Hexagonal Architecture** (Ports & Adapters):

- **Domain layer** (`internal/domain/`) — pure Go business logic with no framework dependencies. Each resource (user, profile, article, comment, tag) has its own controller and repository interface.
- **HTTP inbound adapter** (`internal/adapters/in/webserver/`) — Gorilla Mux HTTP server. Handlers decode requests, call domain services, and encode responses. Authentication is handled by JWT middleware.
- **gRPC inbound adapter** (`internal/adapters/in/grpc/`) — native gRPC server backed by proto-generated stubs. A unary interceptor and a separate stream interceptor each handle auth (mandatory, optional, or none) per method. Both servers share the same domain controller instances — no business logic duplication.
- **Outbound adapter** (`internal/adapters/out/db/`) — PostgreSQL persistence via `sqlx`. Goose migrations run automatically on startup.

See [arch.md](arch.md) for a full description of every layer, route, schema, and design decision.

## CI

Every push to `main` runs the GitHub Actions pipeline. It can also be triggered manually via the **Run workflow** button in the Actions tab.

1. **HTTP integration tests** — checks out the [gothinkster/realworld](https://github.com/gothinkster/realworld) spec repo, installs Hurl, and runs the full HTTP API test suite (`make int-tests`).
2. **gRPC integration tests** — runs the Go e2e test suite in `test/grpc/` against a live server and test database (`make int-tests-grpc`).

## How it was developed

Features were written as plain-English specification files (e.g. `features/9-create-article.md`). Each feature was implemented with the assistance of **Claude Code**, an AI coding tool. The workflow for each feature was:

1. Write a feature specification describing the required behaviour.
2. Review and guide Claude Code's implementation plan in `features/plans/`.
3. Review the implementation across all required layers.
4. Verify `make lint` reported no issues and `make int-tests` passed all integration tests.
5. Review updates to `arch.md` to keep the architecture document current.

The observability stack (Prometheus instrumentation, Grafana provisioning, postgres_exporter) was also designed collaboratively with Claude Code.

## gRPC API

The server exposes a native gRPC API alongside the existing HTTP API. The port is configurable via the `GRPC_PORT` environment variable (production default: **8099**, test environment: **8098**). Service definitions live in `api/proto/` and the generated Go stubs are committed to `api/proto/gen/pb/`. To regenerate after editing a `.proto` file:

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

Authenticated RPCs expect an `authorization` metadata key with value `Token <jwt>`. Methods that require authentication (`GetUser`, `UpdateUser`, `FollowUser`, `UnfollowUser`, `CreateArticle`, `UpdateArticle`, `FavoriteArticle`, `UnfavoriteArticle`, `DeleteArticle`, `FeedArticles`, `CreateComment`, `DeleteComment`, `LiveArticleFeed`) return `UNAUTHENTICATED` if the token is absent or invalid. Methods with optional auth (`GetProfile`, `GetArticleBySlug`, `ListArticles`, `GetComments`, `LiveCommentFeed`) proceed unauthenticated if no token is supplied. `RegisterUser`, `LoginUser`, and `GetTags` require no token.

Unary and server-streaming RPCs are authenticated by separate interceptors. The unary `AuthInterceptor` handles all request/response RPCs. The `StreamAuthInterceptor` handles the two streaming RPCs (`LiveArticleFeed`, `LiveCommentFeed`) using the same three-level scheme — it wraps the `ServerStream` to propagate the enriched context (with `UserIDKey` set) to the handler.

**Structured errors**

Every error returned by the gRPC server carries a standard [`google.rpc.Status`](https://github.com/googleapis/googleapis/blob/master/google/rpc/status.proto) with one or more typed detail messages attached, so clients can inspect structured fields rather than parsing the string message:

| Domain error | gRPC code | Detail type | Key fields |
|---|---|---|---|
| Validation failure | `INVALID_ARGUMENT` | `google.rpc.BadRequest` | `field_violations[].field`, `field_violations[].description` |
| Duplicate field | `ALREADY_EXISTS` | `google.rpc.BadRequest` | `field_violations[].field`, `field_violations[].description` |
| Bad credentials | `UNAUTHENTICATED` | `google.rpc.ErrorInfo` | `reason: "INVALID_CREDENTIALS"`, `domain: "realworld"` |
| Profile not found | `NOT_FOUND` | `google.rpc.ResourceInfo` | `resource_type: "profile"` |
| Article not found | `NOT_FOUND` | `google.rpc.ResourceInfo` | `resource_type: "article"` |
| Comment not found | `NOT_FOUND` | `google.rpc.ResourceInfo` | `resource_type: "comment"` |
| Forbidden | `PERMISSION_DENIED` | `google.rpc.ErrorInfo` | `reason: "PERMISSION_DENIED"`, `domain: "realworld"` |

The mapping lives in `internal/adapters/in/grpc/errors.go`. All four handler files (`user.go`, `article.go`, `profile.go`, `comment.go`) call the single `domainErr` helper instead of building status errors inline.

**Proto3 limitations vs the HTTP API**

- **`UpdateUser` — bio and image use a `NullableString` wrapper.** `optional string` cannot represent the three states needed (absent = leave unchanged, null = clear, value = set). Both fields use `optional NullableString` instead: omit the field to leave it unchanged, send `bio: {}` to clear it to null, or send `bio: { value: "hello" }` to set a value.
- **`UpdateArticle` — tag list uses a `TagListValue` wrapper.** `repeated string` cannot distinguish absent from empty. The field uses `optional TagListValue` instead: omit to leave tags unchanged, send `tag_list: {}` to clear them, or send `tag_list: { tags: ["go"] }` to replace them.

## Running the app

**Prerequisites:** Docker

```bash
make start
```

Starts the full stack in the background: PostgreSQL, the Go server (port **8090** HTTP, **8099** gRPC), Prometheus (port **9090**), and Grafana (port **3000**). The app waits for the database healthcheck to pass before starting.

| Endpoint | URL |
|---|---|
| REST API | http://localhost:8090/api |
| Prometheus metrics | http://localhost:8090/metrics |
| Prometheus UI | http://localhost:9090 |
| Grafana | http://localhost:3000 |

```bash
docker compose down
```

Stops and removes all containers.

## Running with hot reload

**Prerequisites:** Docker

```bash
make dev
```

Same stack as `make start`, but the `app` container runs `air` against a bind-mount of the project root. Saving any `.go` file triggers an automatic recompile and restart inside the container. The Go module cache is persisted in a named volume so dependencies are not re-downloaded on each restart.

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

## Running the gRPC integration tests

**Prerequisites:** Docker, Go 1.21+

```bash
make int-tests-grpc
```

This will:
1. Start a dedicated test database on port 8096
2. Build and start the server against the test environment (ports 8097/8098)
3. Run the full gRPC test suite in `test/grpc/`
4. Truncate the test database and tear it down

The suite covers all gRPC endpoints across ten test files:

| File | What it tests |
|---|---|
| `auth_test.go` | Register, login, get user, update bio/image/username/email, nullable field semantics |
| `articles_test.go` | Create, list (all/by-author/by-tag), get, update, tag list patch, delete |
| `comments_test.go` | Create, list (authed/unauthed), delete, selective deletion |
| `profiles_test.go` | Get profile (authed/unauthed), follow, unfollow, persist check |
| `tags_test.go` | Create article with tags, verify tags appear in `GetTags` |
| `feed_test.go` | Empty feed, follow author, feed count/author, pagination |
| `favorites_test.go` | Favorite, get as favoriter/non-favoriter, list by favorited, unfavorite |
| `pagination_test.go` | Limit/offset combinations, empty page total count, most-recent-first order |
| `errors_test.go` | Missing fields, duplicates, wrong password, `NotFound`, `PermissionDenied`, `Unauthenticated` |
| `streaming_test.go` | `LiveArticleFeed` (auth required, filters to followed authors); `LiveCommentFeed` authenticated (`following: true` for followed authors) and unauthenticated (`following: false`), plus per-slug isolation |

### Why Go instead of shell scripts or Bruno

The gRPC test suite started as shell scripts using `grpcurl` piped into `jq`. This turned out to be the wrong tool for the job for several reasons:

**Proto3 zero-value omission.** gRPC JSON encoding omits fields set to their zero value — `false` booleans and `0` integers simply don't appear in the JSON output. Every boolean or counter assertion required a `// false` or `// 0` jq fallback to avoid silent false-positives, and any assertion that didn't have one would incorrectly pass.

**Shell fragility.** Two separate bugs surfaced within the first test run:
- `UID` is a read-only variable in bash and zsh; using it as a test identifier caused every test to fail immediately with `UID: readonly variable`.
- `${2:-{}}` (a common pattern for defaulting a missing argument to `{}`) silently appends a spurious `}` to the argument when it is set, because `}` closes the parameter expansion before the literal `}` is consumed. This made every JSON request body malformed and every `grpcurl` call fail silently.

**Bruno** requires the desktop application for the full authoring experience and has limited CI integration. Its collection format is designed around HTTP; gRPC support is present but less complete.

**Go** solves all of this cleanly. The generated proto client stubs use native Go types, so zero values (`false`, `0`) are just zero values — no JSON serialization workarounds. `t.Fatalf`, `t.Cleanup`, and build tags (`//go:build integration`) are standard. The compiler catches type mismatches against the proto contract before the test even runs. The tests live in the same repository and run with `go test`, requiring no external binaries beyond the running server.

## Running the linter

```bash
make lint
```
