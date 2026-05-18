# Architecture

## Overview

A [RealWorld](https://github.com/gothinkster/realworld) backend implementation in Go using **Hexagonal Architecture** (Ports & Adapters). Business logic is isolated in a domain layer, with adapters for HTTP input and PostgreSQL output.

## Project Structure

```
realworld-backend-go/
├── cmd/server/server.go              # Entry point — wires domain, HTTP, and gRPC
├── api/proto/                        # Protobuf service definitions
│   ├── user.proto
│   ├── article.proto
│   ├── comment.proto
│   ├── profile.proto
│   ├── tag.proto
│   ├── buf.yaml                      # buf build config
│   ├── buf.lock
│   └── gen/pb/                       # Generated Go stubs (committed)
├── internal/
│   ├── domain/                       # Business logic (no external dependencies)
│   │   ├── models.go                 # Core data models
│   │   ├── user.go                   # UserController + userRepo interface
│   │   ├── profile.go                # ProfileController + profileRepo interface
│   │   ├── article.go                # ArticleController + articleRepo interface
│   │   ├── tag.go                    # TagController + tagRepo interface
│   │   ├── comment.go                # CommentController + commentRepo interface
│   │   └── errors.go                 # Domain error types
│   └── adapters/
│       ├── in/
│       │   ├── webserver/            # Inbound: HTTP
│       │   │   ├── server.go         # Gorilla Mux router setup
│       │   │   └── handlers.go       # HTTP request/response handling
│       │   └── grpc/                 # Inbound: gRPC
│       │       ├── server.go         # Registers all service servers + reflection
│       │       ├── middleware.go     # AuthInterceptor (NoAuth/OptionalAuth/MandatoryAuth)
│       │       ├── user.go           # UserServer
│       │       ├── article.go        # ArticleServer
│       │       ├── profile.go        # ProfileServer
│       │       ├── comment.go        # CommentServer
│       │       └── tag.go            # TagServer
│       └── out/db/                   # Outbound: PostgreSQL
│           ├── postgres.go           # sqlx-based repository
│           └── migrations/
│               ├── 001_create_users.sql
│               ├── 002_unique_users.sql
│               ├── 003_create_follows.sql
│               ├── 004_create_articles.sql
│               ├── 005_create_tags.sql
│               ├── 006_create_article_favorites.sql
│               ├── 007_create_comments.sql
│               └── 008_allow_duplicate_article_titles.sql
├── test/grpc/                        # gRPC e2e integration tests (//go:build integration)
│   ├── helpers_test.go               # dial, withToken, genUID, nullableStr helpers
│   ├── auth_test.go
│   ├── articles_test.go
│   ├── comments_test.go
│   ├── profiles_test.go
│   ├── tags_test.go
│   ├── feed_test.go
│   ├── favorites_test.go
│   ├── pagination_test.go
│   └── errors_test.go
├── compose.yaml                      # Docker Compose (prod DB)
├── compose.test.yaml                 # Docker Compose (test DB)
├── Makefile                          # make int-tests / make int-tests-grpc
├── .env                              # Production environment config
└── .env_test                         # Test environment config
```

## Layers

### Domain (`internal/domain/`)
Pure Go with no framework dependencies. Contains:
- **`UserController`**: Orchestrates user registration — validates input, hashes password with Argon2ID, calls the repository, returns a domain `User`. JWTs use the user's immutable integer `ID` as the `sub` claim (stored as a decimal string per the JWT spec).
- **`userRepo` interface**: Decouples domain from persistence. The DB adapter implements this. Key methods: `InsertUser`, `GetUserByEmail`, `GetUserByID`, `GetFullUserByID`, `UpdateUser`.
- **`ProfileController`**: Handles profile lookups and follow/unfollow operations. Methods: `GetProfile(ctx, profileUsername, viewerID)`, `FollowUser(ctx, followerID, followeeUsername)`, `UnfollowUser(ctx, followerID, followeeUsername)`. Returns actual `following` status.
- **`profileRepo` interface**: Decouples profile domain from persistence. Methods: `GetProfileByUsername(ctx, profileUsername, viewerID)`, `FollowUser(ctx, followerID, followeeUsername)`, `UnfollowUser(ctx, followerID, followeeUsername)`.
- **`ValidationError`**: Structured field-level validation errors for HTTP consumers.
- **`DuplicateError`**: Error type returned when a unique constraint is violated. Carries the `Field` name and a fixed message (`"has already been taken"`).
- **`CredentialsError`**: Error type returned when login credentials are invalid (wrong password or unknown email).
- **`ProfileNotFoundError`**: Error type returned when a profile lookup finds no matching user.
- **`ArticleNotFoundError`**: Error type returned when an article lookup finds no matching article.
- **`CommentNotFoundError`**: Error type returned when a comment lookup finds no matching comment (or the comment doesn't belong to the specified article).
- **`ForbiddenError`**: Error type returned when an authenticated user attempts an action on a resource they don't own (e.g., editing or deleting another user's article or comment). Handlers map this to HTTP 403 with a resource-specific `"forbidden"` error body.
- **`ArticleController`**: Handles article creation, retrieval, listing, feed, updates, favorites, and deletion. Validates input, deduplicates tags (first-occurrence wins), generates slug from title via exported `GenerateSlug(title)` (kebab-case regex). Methods: `CreateArticle(ctx, authorID, a)`, `GetArticleBySlug(ctx, slug, viewerID)`, `ListArticles(ctx, filter, viewerID)`, `FeedArticles(ctx, filter, viewerID)`, `UpdateArticle(ctx, callerID, slug, u)`, `FavoriteArticle(ctx, userID, slug)`, `UnfavoriteArticle(ctx, userID, slug)`, `DeleteArticle(ctx, callerID, slug)`.
- **`articleRepo` interface**: Decouples article domain from persistence. Methods: `InsertArticle(ctx, authorID, slug, a)`, `GetArticleBySlug(ctx, slug, viewerID)`, `ListArticles(ctx, filter, viewerID)`, `FeedArticles(ctx, filter, viewerID)`, `UpdateArticle(ctx, callerID, slug, u)`, `FavoriteArticle(ctx, userID, slug)`, `UnfavoriteArticle(ctx, userID, slug)`, `DeleteArticle(ctx, callerID, slug)`.
- **`ListArticlesFilter`**: Input model for article listing. Fields: `Tag`, `Author`, `Favorited` (`*string`, all optional/case-insensitive), `Limit` (default 20), `Offset` (default 0).
- **`ArticleFeedFilter`**: Input model for the article feed endpoint. Fields: `Limit` (default 20), `Offset` (default 0).
- **`ArticleList`**: Return type for article listing. Fields: `Articles []*Article`, `TotalCount int` (total matching count before limit/offset).
- **`UpdateArticle`**: Extended with `TagList *[]string` — `nil` means preserve existing tags; non-nil (including empty slice) replaces them.
- **`TagController`**: Handles tag listing. Method: `GetTags(ctx)`.
- **`tagRepo` interface**: Decouples tag domain from persistence. Method: `GetAllTags(ctx)`.
- **`CommentController`**: Handles comment creation, retrieval, and deletion. Validates body is non-blank on creation. Methods: `CreateComment(ctx, authorID, articleSlug, c)`, `GetComments(ctx, articleSlug, viewerID)`, `DeleteComment(ctx, callerID, articleSlug, commentID)`.
- **`commentRepo` interface**: Decouples comment domain from persistence. Methods: `InsertComment(ctx, authorID, articleSlug, c)`, `GetCommentsByArticleSlug(ctx, articleSlug, viewerID)`, `DeleteComment(ctx, callerID, articleSlug, commentID)`.

### Inbound Adapter — HTTP (`internal/adapters/in/webserver/`)
Handles the HTTP protocol layer:
- **Router**: Gorilla Mux with Gorilla Handlers for Apache-style request logging. Protected routes are grouped in a subrouter with `authMiddleware` applied.
- **Middleware**: `authMiddleware(jwtSecret)` extracts the JWT from the `Authorization: Token {jwt}` header (401 if missing), validates the signature and expiry (401 if invalid), parses the `sub` claim as an integer user ID (401 if not a valid integer), and stores it in the request context under `userIDKey`. Protected handlers read the user ID from context and pass it directly to the domain service. `optionalAuthMiddleware(jwtSecret)` performs the same validation but silently ignores absent or invalid tokens — used for routes where authentication is optional.
- **Handlers**: Decode JSON, map to domain models, call domain services, encode responses.
- **DTOs**: `RegisterUserRequest` / `UserResponse` wrap payloads in `{"user": {...}}` per the RealWorld spec.

**Current routes:**
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/users` | Register a new user |
| POST | `/api/users/login` | Log in an existing user |
| GET | `/api/user` | Get the authenticated user |
| PUT | `/api/user` | Update the authenticated user |
| GET | `/api/profiles/{username}` | Get a user's public profile (auth optional) |
| POST | `/api/profiles/{username}/follow` | Follow a user (auth required) |
| DELETE | `/api/profiles/{username}/follow` | Unfollow a user (auth required) |
| POST | `/api/articles` | Create an article (auth required) |
| GET | `/api/articles` | List articles with optional filters (auth optional) |
| GET | `/api/articles/feed` | Feed: articles by followed users (auth required) |
| GET | `/api/articles/{slug}` | Get an article by slug (auth optional) |
| PUT | `/api/articles/{slug}` | Update an article (auth required, author only) |
| DELETE | `/api/articles/{slug}` | Delete an article (auth required, author only) |
| POST | `/api/articles/{slug}/favorite` | Favorite an article (auth required) |
| DELETE | `/api/articles/{slug}/favorite` | Unfavorite an article (auth required) |
| POST | `/api/articles/{slug}/comments` | Create a comment on an article (auth required) |
| GET | `/api/articles/{slug}/comments` | Get comments for an article (auth optional) |
| DELETE | `/api/articles/{slug}/comments/{id}` | Delete a comment (auth required, author only) |
| GET | `/api/tags` | List all tags (no auth) |

**Response codes:** `200 OK`, `201 Created`, `401 Unauthorized`, `403 Forbidden`, `404 Not Found`, `409 Conflict`, `422 Unprocessable Entity`, `500 Internal Server Error`

### Inbound Adapter — gRPC (`internal/adapters/in/grpc/`)

Runs on a separate port (`GRPC_PORT`) alongside the HTTP server. Both servers are started concurrently from `cmd/server/server.go`; both delegate to the same domain controller instances, so there is no business logic duplication.

**`server.go`** — `NewGrpcServer` registers all five service servers (`UserServiceServer`, `ArticleServiceServer`, `ProfileServiceServer`, `CommentServiceServer`, `TagServiceServer`) and enables gRPC reflection so tools like `grpcurl` can discover the API at runtime.

**`middleware.go`** — `AuthInterceptor` is a unary server interceptor that implements a three-level auth scheme:

| Constant | Behaviour |
|---|---|
| `NoAuth` | Token is not read at all (e.g. `RegisterUser`, `LoginUser`, `GetTags`) |
| `OptionalAuth` | Token is parsed and the user ID is placed in context if valid; the call proceeds regardless (e.g. `GetProfile`, `GetArticleBySlug`, `ListArticles`, `GetComments`) |
| `MandatoryAuth` | Returns `UNAUTHENTICATED` if the token is absent or invalid |

The per-method requirements are declared as a `map[string]AuthRequirement` in `cmd/server/server.go` and passed to the interceptor at server construction. The interceptor extracts the JWT from the `authorization` metadata key (format: `Token <jwt>`), validates it with the shared JWT secret, parses the `sub` claim as an integer user ID, and stores it in the request context under `UserIDKey`. Handlers read the ID with `ctx.Value(UserIDKey).(int)`; for optional-auth handlers they use the zero value (unauthenticated) if the key is absent.

**Service servers** — each file defines a narrow service interface (local to the package) and a server struct that implements the generated proto service interface:

| File | Server | Domain errors → gRPC codes |
|---|---|---|
| `user.go` | `UserServer` | `ValidationError` → `InvalidArgument`, `DuplicateError` → `AlreadyExists`, `CredentialsError` → `Unauthenticated` |
| `article.go` | `ArticleServer` | `ArticleNotFoundError` → `NotFound`, `ForbiddenError` → `PermissionDenied`, `ValidationError` → `InvalidArgument` |
| `profile.go` | `ProfileServer` | `ProfileNotFoundError` → `NotFound` |
| `comment.go` | `CommentServer` | `ArticleNotFoundError` / `CommentNotFoundError` → `NotFound`, `ForbiddenError` → `PermissionDenied`, `ValidationError` → `InvalidArgument` |
| `tag.go` | `TagServer` | — |

**Proto3 wrapper types** (defined in the `.proto` files, generated into `api/proto/gen/pb/`):
- `NullableString` — used for `UpdateUser.bio` and `UpdateUser.image` to represent three states: field absent (leave unchanged), `{}` (clear to null), `{ value: "..." }` (set a value). Plain `optional string` cannot represent this because proto3 cannot distinguish absent from empty-string.
- `TagListValue` — used for `UpdateArticle.tag_list` to distinguish absent (preserve tags) from `{}` (clear all tags) from `{ tags: ["go"] }` (replace tags). Plain `repeated string` cannot represent absent.

**`article.go` helpers** — shared private functions keep the handlers thin:
- `articleToProto` / `articleListItemToProto` — convert domain `Article` to the single-article and list-item proto shapes respectively.
- `articleAuthorToProto` — converts a domain `Profile` to `ArticleAuthor`.
- `articlesResponse` — builds the `ArticlesResponse` (list + total count) from a domain `ArticleList`.
- `articleErr` — maps `ArticleNotFoundError` / `ForbiddenError` to the correct gRPC status; falls through to `Internal` for unexpected errors.

### Outbound Adapter — Database (`internal/adapters/out/db/`)
PostgreSQL persistence via `sqlx`:
- Runs embedded Goose migrations automatically on startup.
- Parameterized queries to prevent SQL injection.
- `InsertUser()` inserts a user and returns the created record via `RETURNING`. Detects PostgreSQL unique-violation errors (code `23505`) and maps them to `*domain.DuplicateError` by constraint name.
- `GetUserByEmail()` fetches a user and their hashed password by email. Returns `*domain.CredentialsError` when no row is found.
- `GetUserByID()` fetches a user by ID. Returns `*domain.CredentialsError` when no row is found.
- `GetFullUserByID()` fetches a user and their hashed password by ID. Returns `*domain.CredentialsError` when no row is found.
- `UpdateUser()` updates all user fields by user ID and returns the updated record via `RETURNING`. Maps unique-violation errors to `*domain.DuplicateError`.
- `GetProfileByUsername(ctx, profileUsername, viewerID)` fetches a user's public profile fields by username using a LEFT JOIN on `follows` to compute the real `following` status for the viewer. Pass `viewerID=0` for unauthenticated requests. Returns `*domain.ProfileNotFoundError` when no row is found.
- `FollowUser(ctx, followerID, followeeUsername)` inserts a row into `follows` (idempotent via `ON CONFLICT DO NOTHING`) then calls `GetProfileByUsername` to return the full profile. Returns `*domain.ProfileNotFoundError` when the followee username does not exist.
- `UnfollowUser(ctx, followerID, followeeUsername)` deletes the corresponding `follows` row then calls `GetProfileByUsername` to return the full profile. Returns `*domain.ProfileNotFoundError` when the followee username does not exist.
- `InsertArticle(ctx, authorID, slug, a)` wraps all operations in a transaction. Before inserting, finds a unique slug by looping with a `SELECT EXISTS` check: tries `slug`, then `slug-2`, `slug-3`, … until no collision (duplicate titles are allowed; only slugs must be unique). Inserts the article, upserts tags (via `INSERT ... ON CONFLICT DO NOTHING`), links tags to the article via `article_tags`, then fetches the author profile. Returns `TagList` from the (deduplicated) input; `Favorited` is always `false`, `FavoritesCount` is always `0`.
- `GetArticleBySlug(ctx, slug, viewerID)` fetches a single article by slug in one query: JOINs `users` for the author, LEFT JOINs `follows` for `following`, LEFT JOINs `article_tags`+`tags` with `ARRAY_AGG` for tags, LEFT JOINs `article_favorites` for viewer-specific `favorited`, and uses a correlated subquery for `favoritesCount`. All viewer-specific fields use `viewerID=0` → `false` for unauthenticated requests. Returns `*domain.ArticleNotFoundError` when no row is found.
- `FavoriteArticle(ctx, userID, slug)` inserts into `article_favorites` (`ON CONFLICT DO NOTHING`) then calls `GetArticleBySlug`. Returns `*domain.ArticleNotFoundError` if the slug doesn't exist.
- `UnfavoriteArticle(ctx, userID, slug)` deletes from `article_favorites` then calls `GetArticleBySlug`. Returns `*domain.ArticleNotFoundError` if the slug doesn't exist.
- `InsertComment(ctx, authorID, articleSlug, c)` uses a single CTE query: `INSERT INTO comments ... SELECT ... FROM articles WHERE slug = $3` — if the article doesn't exist, 0 rows are inserted and `sql.ErrNoRows` from `RETURNING` maps to `*domain.ArticleNotFoundError`. Joins `users` in the same query to return the author profile.
- `GetCommentsByArticleSlug(ctx, articleSlug, viewerID)` first checks article existence (→ `ArticleNotFoundError` if missing), then queries all comments with a JOIN on `users` and LEFT JOIN on `follows` for viewer-specific `following`. Returns `[]*domain.Comment` ordered by `created_at ASC`; returns an empty slice (never nil) when the article exists but has no comments.
- `DeleteComment(ctx, callerID, articleSlug, commentID)` checks article existence (→ `ArticleNotFoundError`), checks comment existence with matching `article_id` (→ `CommentNotFoundError`), checks `author_id == callerID` (→ `ForbiddenError`), then deletes the comment.
- `UpdateArticle(ctx, callerID, slug, u)` wraps the update in a transaction: fetches the current article (→ `ArticleNotFoundError` if missing), checks `author_id == callerID` (→ `ForbiddenError` if not), merges partial fields. If title changed, finds a unique new slug via `SELECT EXISTS` loop (`slug`, `slug-2`, …, excluding the current article's own ID). Runs `UPDATE`. If `u.TagList != nil`, clears existing `article_tags` and re-inserts the new set. Commits, then calls `GetArticleBySlug` to return the full response.
- `ListArticles(ctx, filter, viewerID)` and `FeedArticles(ctx, filter, viewerID)` both delegate to a shared private `buildArticleListQuery` helper that constructs the common SELECT/FROM/JOIN/GROUP BY/ORDER BY/LIMIT/OFFSET query. `ListArticles` adds optional WHERE conditions for `tag`, `author`, and `favorited`. `FeedArticles` adds `f.follower_id IS NOT NULL` to restrict results to articles by followed authors. Both use a shared `listRow` struct and `convertListRows` helper. Both omit the `body` column and return `*domain.ArticleList` (never nil, Articles slice never nil).
- `DeleteArticle(ctx, callerID, slug)` fetches `author_id` by slug (→ `ArticleNotFoundError` if missing), checks `author_id == callerID` (→ `ForbiddenError` if not), then deletes the article. Cascade constraints on `article_tags`, `article_favorites`, and `comments` handle related row cleanup automatically.
- `GetAllTags(ctx)` returns all tag names ordered alphabetically. Returns `[]string{}` (never nil) when there are no tags.

**Schema (`users` table):**
| Column | Type | Notes |
|--------|------|-------|
| id | SERIAL | Primary key |
| username | VARCHAR(45) | Required, unique |
| email | VARCHAR(45) | Required, unique |
| password | VARCHAR(100) | Argon2ID hash |
| bio | TEXT | Optional |
| image | VARCHAR(100) | Optional (profile picture URL) |

**Schema (`follows` table):**
| Column | Type | Notes |
|--------|------|-------|
| follower_id | INTEGER | FK → users.id, part of PK |
| followee_id | INTEGER | FK → users.id, part of PK |

**Schema (`articles` table):**
| Column | Type | Notes |
|--------|------|-------|
| id | SERIAL | Primary key |
| slug | VARCHAR(255) | Required, unique |
| title | VARCHAR(255) | Required |
| description | TEXT | Required |
| body | TEXT | Required |
| author_id | INTEGER | FK → users.id |
| created_at | TIMESTAMPTZ | Auto-set to now() |
| updated_at | TIMESTAMPTZ | Auto-set to now() |

**Schema (`tags` table):**
| Column | Type | Notes |
|--------|------|-------|
| id | SERIAL | Primary key |
| name | VARCHAR(255) | Required, unique |

**Schema (`article_tags` table):**
| Column | Type | Notes |
|--------|------|-------|
| article_id | INTEGER | FK → articles.id ON DELETE CASCADE, part of PK |
| tag_id | INTEGER | FK → tags.id ON DELETE CASCADE, part of PK |

**Schema (`article_favorites` table):**
| Column | Type | Notes |
|--------|------|-------|
| user_id | INTEGER | FK → users.id ON DELETE CASCADE, part of PK |
| article_id | INTEGER | FK → articles.id ON DELETE CASCADE, part of PK |

**Schema (`comments` table):**
| Column | Type | Notes |
|--------|------|-------|
| id | SERIAL | Primary key |
| body | TEXT | Required |
| author_id | INTEGER | FK → users.id ON DELETE CASCADE |
| article_id | INTEGER | FK → articles.id ON DELETE CASCADE |
| created_at | TIMESTAMPTZ | Auto-set to now() |
| updated_at | TIMESTAMPTZ | Auto-set to now() |

## Key Dependencies

| Package | Purpose |
|---------|---------|
| `gorilla/mux` | HTTP routing |
| `gorilla/handlers` | HTTP request logging |
| `jmoiron/sqlx` | SQL toolkit with row scanning |
| `lib/pq` | PostgreSQL driver |
| `pressly/goose/v3` | Database migrations |
| `alexedwards/argon2id` | Password hashing |
| `golang-jwt/jwt/v5` | JWT token generation (HS256) |
| `joho/godotenv` | `.env` file loading |

## Configuration

Loaded from `.env` / `.env_test` via `godotenv`:

| Variable | Default | Notes |
|----------|---------|-------|
| `SERVER_PORT` | 8090 | HTTP server port (test: 8097) |
| `GRPC_PORT` | 8099 | gRPC server port (test: 8098); required, server exits if missing |
| `JWT_SECRET` | — | HMAC signing key for JWT tokens |
| `DB_HOST` | localhost | PostgreSQL host |
| `DB_PORT` | 8095 | PostgreSQL port (test: 8096) |
| `DB_USER` | admin | |
| `DB_PASSWORD` | password | |
| `DB_NAME` | app | Database name (test: test-app) |

## Infrastructure (Docker)

Docker Compose is split into two files:
- **`compose.yaml`**: Production database (`db`, port 8095, volume `app-data`)
- **`compose.test.yaml`**: Test database (`test_db`, port 8096, volume `app-test-data`)

Migrations run automatically at app startup via embedded Goose files.

## Testing

### HTTP integration tests (`make int-tests`)

1. Starts the test DB (`compose.test.yaml`)
2. Polls `pg_isready` until the DB is accepting connections
3. Builds the binary (`go build ./cmd/server`)
4. Starts the server with the test env (`./server -env .env_test`, HTTP port 8097)
5. Runs the full RealWorld Hurl API test suite against the gothinkster/realworld spec
6. Truncates the `users` table and stops the test DB container

### gRPC e2e tests (`make int-tests-grpc`)

1. Starts the test DB (`compose.test.yaml`)
2. Builds the binary and starts the server (HTTP 8097, gRPC 8098)
3. Runs `go test -tags integration ./test/grpc/` with `GRPC_HOST=localhost:8098`
4. Kills the server, truncates the DB, and tears down the container

The gRPC test suite lives in `test/grpc/` and uses the `//go:build integration` build tag so it is never compiled by a plain `go test ./...`. Tests use the generated proto client stubs directly — no grpcurl or JSON serialization involved. Each test function is self-contained: it registers its own users and articles, and uses `t.Cleanup` for teardown. The suite covers all RPCs including error cases (`NotFound`, `PermissionDenied`, `Unauthenticated`).

The server accepts a `-env` flag (default `.env`) to select the env file at startup.

## Current State

The project implements user **registration**, **login**, **get current user**, **update current user**, **get profile**, **follow user**, and **unfollow user**. Notable details:
- Registered and logged-in users receive a signed HS256 JWT (claims: `sub`=user ID as decimal string, 72h expiry). Using the immutable user ID means tokens remain valid even if the user changes their username.
- `GET /api/user` and `PUT /api/user` are protected by `authMiddleware`, which centralises both token extraction and JWT validation. The domain service receives the authenticated user ID (int) directly and no longer handles tokens.
- `PUT /api/user` supports partial updates (all fields optional); fetches current values, applies changes, and writes all fields back in one query.
- Future protected routes can be added to the protected subrouter with a single line; optionally-authenticated routes go on the optional-auth subrouter.
- `GET /api/profiles/{username}` returns the real `following` status for an authenticated viewer, or `false` for unauthenticated requests.
- `POST /api/profiles/{username}/follow` and `DELETE /api/profiles/{username}/follow` are protected endpoints that create/remove rows in the `follows` table.
- `POST /api/articles` creates an article; slug is generated from the title (kebab-case). `tagList` is stored in the `tags` and `article_tags` tables and returned in the response. `favorited` and `favoritesCount` are always `false`/`0`.
- `GET /api/tags` returns all tags ordered alphabetically.
- `GET /api/articles/{slug}` returns a single article by slug (auth optional); `author.following` reflects the viewer's follow state.
- `POST /api/articles` allows duplicate titles — each article receives a unique slug. When a slug derived from the title is already taken, a numeric suffix is appended (`slug-2`, `slug-3`, …) via a pre-insert `SELECT EXISTS` loop.
- `PUT /api/articles/{slug}` updates an article's title, description, and/or body (at least one required); title change regenerates the slug (with the same suffix collision avoidance). Only the author may update; returns 403 otherwise.
- `POST /api/articles/{slug}/favorite` and `DELETE /api/articles/{slug}/favorite` mark/unmark an article as a favorite for the caller; both are idempotent and return the updated article.
- `favorited` and `favoritesCount` are now real computed values in all single-article responses.
- `POST /api/articles/{slug}/comments` creates a comment on an article; body is required; returns 201 with the comment and author profile.
- `GET /api/articles/{slug}/comments` returns all comments for an article (auth optional); `author.following` reflects the viewer's follow state. Returns 404 if the article is not found, empty array if it exists but has no comments.
- `DELETE /api/articles/{slug}/comments/{id}` deletes a comment; returns 404 if the article or comment is not found (or comment doesn't belong to the article), 403 if the caller is not the comment author, 204 on success.
- `DELETE /api/articles/{slug}` deletes an article and all related data (via cascade); returns 404 if not found, 403 if the caller is not the article author, 204 on success.
- `GET /api/articles` returns a paginated, filterable list of articles (auth optional). Supports `tag`, `author`, `favorited` (case-insensitive), `limit` (default 20), and `offset` (default 0) query params. Response omits `body`; `articlesCount` is the total matching count (pre-limit).
- `PUT /api/articles/{slug}` now also accepts `tagList` (optional): absent preserves existing tags; `[]` clears all tags; `null` returns 422.
- `GET /api/articles/feed` returns a paginated list of articles authored by users that the authenticated user follows, ordered by most recent first. Supports `limit` (default 20) and `offset` (default 0) query params. Response omits `body`; `articlesCount` is the total matching count (pre-limit). Requires authentication (401 if missing/invalid token).
