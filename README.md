# Gochin

A batteries-included API framework for Go, with Laravel-shaped ergonomics — route files, controllers, services, migrations, seeders — implemented the way Go actually wants them: compile-time checked, reflection-free at dispatch, and with almost no dependencies.

```go
// app/Routes/api.go
func init() {
    router.Register(func(r *router.Router) {
        users := controllers.NewUserController(services.NewUserService())

        v1 := r.Group("/v1")
        v1.Get("/users/{id}", users.Show)
        v1.Post("/users", users.Store, middleware.Auth())
    })
}
```

**What's in the box:** HTTP router · ORM with relationships · migrations & schema builder · seeders · token authentication · middleware · structured logging · file storage & uploads · mail · CLI generators.

**Dependencies:** `cobra`, `godotenv`, `lib/pq`, `x/crypto`, `go-figure`. No web framework, no ORM library — the HTTP layer is stdlib `net/http`.

---

## Table of contents

1. [Install](#1-install)
2. [Quick start](#2-quick-start)
3. [Project structure](#3-project-structure)
4. [Configuration](#4-configuration)
5. [CLI reference](#5-cli-reference)
6. [Routing](#6-routing)
7. [Controllers](#7-controllers)
8. [Services](#8-services)
9. [Request validation](#9-request-validation)
10. [Responses & errors](#10-responses--errors)
11. [Models & the ORM](#11-models--the-orm)
12. [Relationships](#12-relationships)
13. [Migrations](#13-migrations)
14. [Seeders](#14-seeders)
15. [Authentication](#15-authentication)
16. [Middleware](#16-middleware)
17. [Logging](#17-logging)
18. [File storage & uploads](#18-file-storage--uploads)
19. [Static files](#19-static-files)
20. [Mail](#20-mail)
21. [Testing](#21-testing)
22. [Architecture notes](#22-architecture-notes)

---

## 1. Install

**Requirements:** Go 1.25+, PostgreSQL, and `git` (Go's module downloader shells out to it when fetching dependencies, even though you never run git commands yourself).

Gochin is a library plus a scaffolding CLI — you install the CLI once, then generate a new project for each app you build, the same way you'd use `rails new`, `cargo new` or `laravel new`:

```bash
go install github.com/sachinkaru123/gochin/cmd/gochin@latest
```

This puts a `gochin` binary under `$(go env GOPATH)/bin`. That directory isn't always on `$PATH` by default (especially on a fresh machine) — if `gochin: command not found` after installing, add it yourself:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"   # add this line to your ~/.bashrc or ~/.zshrc too
```

You do **not** clone this repository to build an app — it's the framework's own source, not a project template you edit in place.

---

## 2. Quick start

```bash
gochin new myapp
cd myapp

cp .env.example .env          # then edit your database credentials
createdb myapp_db

go run . db migrate           # create the tables
go run . db seed              # add development data
go run . run start            # http://localhost:8080
```

`gochin new` generates a small working app, not an empty shell: a `users` + `auth_tokens` schema, a `User` model, and full register/login/me/logout endpoints, all wired and ready to run.

Verify it works:

```bash
curl localhost:8080/health
curl -X POST localhost:8080/api/v1/auth/login \
  -d '{"email":"ada@gochin.test","password":"password123456"}'
```

From here on, run the CLI as `go run . <command>` from inside your project — it compiles your app's own routes, migrations and seeders into the command, which a globally-installed binary can't do for code it hasn't seen. (`gochin run start`, `gochin db migrate`, etc. also work directly: the installed CLI detects it's inside a Gochin project and delegates to `go run .` for you.)

Useful flags:

```bash
go run . run start --port 3000 --host 0.0.0.0
go run . run start --skip-db-test          # start without checking the database
```

---

## 3. Project structure

A generated project looks like this:

```
main.go          blank-imports your app/Migrations, app/Routes, app/Seeders
go.mod           requires github.com/sachinkaru123/gochin
app/
  Controllers/   HTTP entry points — thin
  Services/      business logic and data access
  Models/        database-backed structs
  Requests/      input payloads + validation
  Middleware/    application middleware
  Routes/        route definitions (multiple files, auto-registered)
  Migrations/    versioned schema changes
  Seeders/       development data
public/          web-accessible assets
storage/
  app/           uploaded files (never served)
  logs/          log files
```

Everything under `app/` follows the registry pattern: a file registers itself from `init()`, and `main.go`'s blank imports are what trigger those `init()`s — add a new file and nothing else needs to change.

The framework itself (this repository) is just `pkg/`: the router, ORM, auth, logging, mail and storage packages your generated project imports. **The one rule that keeps it reusable: nothing in `pkg/` may import anything from an application's `app/`.**

---

## 4. Configuration

Everything is read from environment variables, with a `.env` file loaded automatically at startup.

### Server

| Variable | Default | Purpose |
|---|---|---|
| `GOCHIN_HOST` | `localhost` | Bind address |
| `GOCHIN_PORT` | `8080` | Bind port |
| `GOCHIN_ENV` | `development` | Environment name |
| `GOCHIN_DEBUG` | `true` unless production | Include error detail in responses |
| `GOCHIN_API_PREFIX` | `/api` | Prefix for all API routes |
| `GOCHIN_MAX_BODY_BYTES` | `1048576` | JSON body cap |
| `GOCHIN_HANDLER_TIMEOUT_SECONDS` | `15` | Per-request timeout |
| `GOCHIN_TRUST_PROXY` | `false` | Honour `X-Forwarded-For` |
| `GOCHIN_RATE_LIMIT_PER_MINUTE` | `0` (off) | Global rate limit |
| `GOCHIN_CORS_ORIGINS` | *(empty)* | Comma-separated origins, or `*` |

> **`GOCHIN_DEBUG` in production:** debug mode serializes internal error text to clients. It defaults off when `GOCHIN_ENV=production`; don't force it back on.

> **`GOCHIN_TRUST_PROXY`:** only enable this behind a proxy that *overwrites* `X-Forwarded-For`. Otherwise clients can spoof their own IP and defeat rate limiting.

### Database

| Variable | Default |
|---|---|
| `GOCHIN_DB_HOST` | `localhost` |
| `GOCHIN_DB_PORT` | `5432` |
| `GOCHIN_DB_USER` | `postgres` |
| `GOCHIN_DB_PASSWORD` | *(empty)* |
| `GOCHIN_DB_NAME` | `gochin_db` |
| `GOCHIN_DB_SSL_MODE` | `disable` |
| `GOCHIN_DB_MAX_OPEN_CONNS` | `25` |
| `GOCHIN_DB_MAX_IDLE_CONNS` | `5` |
| `GOCHIN_DB_MAX_LIFETIME` | `5` (minutes) |

> Size `MAX_OPEN_CONNS` at roughly `cores × 2`. Postgres forks a process per connection, so 100 app connections across 3 instances will exhaust `max_connections`. Set `MAX_IDLE_CONNS` equal to `MAX_OPEN_CONNS` to avoid connection churn.

### Auth, logging, storage, mail

| Variable | Default |
|---|---|
| `GOCHIN_AUTH_TOKEN_TTL_HOURS` | `720` (0 = never expires) |
| `GOCHIN_AUTH_IDLE_TIMEOUT_HOURS` | `0` (off) |
| `GOCHIN_LOGIN_RATE_PER_MINUTE` | `5` |
| `GOCHIN_ARGON2_MEMORY_KIB` | `19456` |
| `GOCHIN_ARGON2_TIME` | `2` |
| `GOCHIN_ARGON2_PARALLELISM` | `1` |
| `GOCHIN_ARGON2_MAX_CONCURRENT` | `4` |
| `GOCHIN_LOG_DIR` | `storage/logs` |
| `GOCHIN_LOG_LEVEL` | `debug` dev / `info` production |
| `GOCHIN_LOG_FORMAT` | `text` dev / `json` production |
| `GOCHIN_LOG_TO_FILE` / `GOCHIN_LOG_TO_STDOUT` | `true` |
| `GOCHIN_STORAGE_ROOT` | `storage/app` |
| `GOCHIN_PUBLIC_DIR` / `GOCHIN_PUBLIC_URL` | `public` / `/assets` |
| `GOCHIN_MAX_UPLOAD_BYTES` | `10485760` |
| `GOCHIN_MAIL_DRIVER` | `log` |
| `GOCHIN_MAIL_HOST` / `PORT` | `localhost` / `1025` |
| `GOCHIN_MAIL_USERNAME` / `PASSWORD` | *(empty)* |
| `GOCHIN_MAIL_FROM_ADDRESS` / `FROM_NAME` | `no-reply@gochin.test` / `Gochin` |
| `GOCHIN_MAIL_TLS` | `true` |

---

## 5. CLI reference

```bash
gochin new <name> [--module=path]   # scaffold a new project

gochin run start [--host] [--port] [--skip-db-test]

gochin make controller <Name> [--resource] [--force]
gochin make service    <Name> [--force]
gochin make model      <Name> [--force]
gochin make middleware <Name> [--force]
gochin make migration  <name> [--table=x] [--force]
gochin make seeder     <Name> [--force]

gochin db migrate [--rollback]
gochin db seed [--only=name] [--list]

gochin route list [--all] [--json] [--method GET] [--path /users]
```

`gochin route list` shows every route with its handler and middleware — the fastest way to see what your app exposes:

```
METHOD  PATH                  HANDLER                  MIDDLEWARE
GET     /api/v1/users         UserController.Index     RequestID, Logger, Recover, ..., Required
POST    /api/v1/auth/login    AuthController.Login     RequestID, Logger, ..., RateLimit
```

It runs without a database, so it works in CI.

`gochin new`, `gochin make ...` work anywhere. `run`, `db` and `route` need your app's own compiled-in routes/migrations/seeders: run them as `go run . <command>` from your project root, or just use the installed `gochin` binary directly — it detects a Gochin project (a `go.mod` requiring the framework, next to a `main.go`) and transparently delegates to `go run .` for you.

---

## 6. Routing

### Route files

Every file in `app/Routes` registers itself from `init()`. Add as many files as you like — nothing central needs editing.

```go
// app/Routes/api.go
package routes

import (
    controllers "myapp/app/Controllers"
    middleware "myapp/app/Middleware"
    services "myapp/app/Services"
    "github.com/sachinkaru123/gochin/pkg/router"
)

func init() {
    router.Register(func(r *router.Router) {
        posts := controllers.NewPostController(services.NewPostService())

        v1 := r.Group("/v1")              // → /api/v1

        v1.Get("/posts", posts.Index)
        v1.Post("/posts", posts.Store, middleware.Auth())
    })
}
```

Controllers are constructed **inside** the registrar, not at package level, so no database connection is opened during package initialization.

### Verbs, groups and versioning

```go
v1 := r.Group("/v1")                          // /api/v1
v2 := r.Group("/v2")                          // /api/v2 — versions coexist

users := v1.Group("/users", middleware.Auth()) // group-wide middleware
users.Get("",           ctrl.Index)            // GET    /api/v1/users
users.Get("/{id}",      ctrl.Show)             // GET    /api/v1/users/{id}
users.Post("",          ctrl.Store)
users.Put("/{id}",      ctrl.Update)
users.Delete("/{id}",   ctrl.Destroy)
users.Patch("/{id}",    ctrl.Patch)
users.Handle("OPTIONS", "/{id}", ctrl.Options) // any other verb

r.Root().Get("/health", handler)               // no API prefix
```

The API prefix comes from `GOCHIN_API_PREFIX`; versions are explicit groups, because v1 and v2 usually point at different controllers.

### Handler shape

```go
type Handler func(*router.Context) error
```

Returning an error hands control to the central error renderer, so you never repeat error-response boilerplate — and can't forget a `return` after writing one.

### Path parameters

```go
v1.Get("/users/{id}",       ctrl.Show)     // c.Param("id")
v1.Get("/files/{path...}",  ctrl.Serve)    // matches the rest of the path
```

> **Never put a trailing slash on a route.** In Go's ServeMux a trailing slash makes the pattern a subtree match, so `/users/` would swallow `/users/anything`.

### What you get for free

`Build()` runs once at boot and adds, for every route:

- **JSON 405** with an accurate `Allow` header for verbs you didn't implement
- **`OPTIONS`** returning 204 with the same `Allow`
- **JSON 404** for unmatched paths

Conflicting or malformed routes **panic at startup**, not at request time.

---

## 7. Controllers

Keep them thin: read input, call one service method, shape the response.

```go
// app/Controllers/PostController.go
package controllers

type PostController struct {
    posts *services.PostService
}

func NewPostController(posts *services.PostService) *PostController {
    return &PostController{posts: posts}
}

func (c *PostController) Show(ctx *router.Context) error {
    id, err := ctx.ParamInt("id")
    if err != nil {
        return err                              // → 400
    }

    post, err := c.posts.Get(ctx.Ctx(), id)
    if err != nil {
        return err                              // → 404 if not found
    }
    return ctx.JSON(http.StatusOK, post)
}
```

Generate one with CRUD stubs:

```bash
gochin make controller Post --resource
```

### Context API

| Reading input | |
|---|---|
| `c.Bind(&dto)` | Decode JSON body (size-capped, rejects unknown fields) |
| `c.Param("id")` / `c.ParamInt("id")` | Path wildcard |
| `c.Query("q", "default")` / `c.QueryInt("page", 1)` | Query string |
| `c.FormFile("avatar")` | Uploaded file |
| `c.Ctx()` | Request context — pass to services |
| `c.ClientIP()`, `c.RequestID()`, `c.Header()` | Request metadata |

| Writing output | |
|---|---|
| `c.JSON(200, v)` | JSON response |
| `c.String(200, "hi %s", name)` | Plain text |
| `c.HTML(200, body)` | HTML |
| `c.NoContent(204)` | Empty |
| `c.Redirect(302, url)` | Redirect |

> `*router.Context` is pooled and reused. Never retain it — or anything derived from its `Request` — after the handler returns.

---

## 8. Services

Business logic and data access. Services take a `context.Context`, never a `*router.Context`, so they're reusable from CLI commands and background jobs.

```go
// app/Services/PostService.go
package services

type PostService struct{}

func NewPostService() *PostService { return &PostService{} }

func (s *PostService) Get(ctx context.Context, id int64) (*models.Post, error) {
    return orm.FindCtx[models.Post](ctx, id)
}

func (s *PostService) List(ctx context.Context, page, perPage int) (*orm.Page[models.Post], error) {
    return orm.Query[models.Post]().
        WithContext(ctx).
        OrderBy("id", "DESC").
        Paginate(page, perPage)
}
```

Keep services **stateless**. The ORM resolves the database connection lazily per call, so a constructor never needs to open one — which is also what lets `gochin route list` run without a database.

```bash
gochin make service Post
```

---

## 9. Request validation

```go
// app/Requests/PostRequests.go
type CreatePost struct {
    Title  string `json:"title"`
    UserID int64  `json:"user_id"`
}

func (r CreatePost) Validate() error {
    fields := map[string]string{}

    if strings.TrimSpace(r.Title) == "" {
        fields["title"] = "title is required"
    }
    if r.UserID <= 0 {
        fields["user_id"] = "user_id is required"
    }

    if len(fields) > 0 {
        return router.Unprocessablef("validation failed").WithFields(fields)
    }
    return nil
}
```

Produces:

```json
{ "error": {
    "code": "unprocessable_entity",
    "message": "validation failed",
    "fields": { "title": "title is required" }
} }
```

---

## 10. Responses & errors

Return an error from anywhere and it renders consistently:

```go
return router.NotFoundf("user %d not found", id)   // 404
return router.BadRequestf("invalid cursor")        // 400
return router.Unauthorizedf("token required")      // 401
return router.Forbiddenf("not your resource")      // 403
return router.Conflictf("email already taken")     // 409
return router.Unprocessablef("validation failed")  // 422
return router.Internalf("upstream failed")         // 500
return router.NewError(418, "i am a teapot")       // anything
```

Chainable detail:

```go
return router.Conflictf("duplicate").
    WithCode("duplicate_email").
    WithFields(map[string]string{"email": "already registered"}).
    Wrap(err)                                      // logged, never serialized
```

Envelope:

```json
{ "error": {
    "code": "not_found",
    "message": "user 42 not found",
    "request_id": "9c67cff8230fe71c"
} }
```

### Domain errors → status codes

`bootstrap/errors.go` maps errors centrally, so services can return raw errors:

| Error | Status |
|---|---|
| `orm.ErrRecordNotFound`, `sql.ErrNoRows` | 404 |
| `orm.ErrMissingPrimaryKey` | 400 |
| Postgres `23505` unique violation | **409** |
| Postgres `23503` foreign key violation | 400 |
| Postgres `23502` not-null violation | 400 |
| `context.DeadlineExceeded`, Postgres `57014` | 504 |
| `context.Canceled` | 499 |
| Postgres `53300` too many connections | 503 |
| anything else | 500 (detail never leaked) |

Add your own:

```go
router.RegisterErrorMapper(func(err error) *router.HTTPError {
    if errors.Is(err, services.ErrInsufficientFunds) {
        return router.Unprocessablef("insufficient funds")
    }
    return nil
})
```

---

## 11. Models & the ORM

### Defining a model

```go
// app/Models/Post.go
type Post struct {
    orm.Model                                        // ID, CreatedAt, UpdatedAt

    Title  string `db:"title" json:"title"`
    Body   string `db:"body" json:"body"`
    UserID int64  `db:"user_id" json:"user_id"`

    Author *User `db:"-" json:"author,omitempty"`    // relation, not a column
}

func (Post) TableName() string { return "posts" }
```

**Tags:**

| Tag | Meaning |
|---|---|
| `db:"title"` | Column name |
| `db:"id,primary_key"` | Primary key |
| `db:"-"` | **Not a column** — required on relation fields |
| *(no tag)* | Defaults to `snake_case` of the field name |

> `TableName()` must use a **value receiver** (`func (Post)`, not `func (*Post)`). That's what lets `orm.Find[Post](id)` compile.

```bash
gochin make model Post
```

### CRUD

```go
post, err := orm.FindCtx[models.Post](ctx, 42)
all,  err := orm.AllCtx[models.Post](ctx)

err = orm.CreateCtx(ctx, &post)   // fills ID, CreatedAt, UpdatedAt
err = orm.UpdateCtx(ctx, &post)   // refreshes UpdatedAt
err = orm.DeleteCtx(ctx, &post)
```

Non-`Ctx` variants exist (`orm.Find`, `orm.Create`, …) for CLI and scripts. **In request handling always use the `Ctx` form** — it propagates cancellation to Postgres, so a client that disconnects doesn't leave a query holding a pooled connection.

### Query builder

```go
posts, err := orm.Query[models.Post]().
    WithContext(ctx).
    Where("published", "=", true).
    Where("views", ">", 100).
    WhereIn("category_id", []any{1, 2, 3}).
    OrderBy("created_at", "DESC").
    Limit(10).
    Offset(20).
    Get()

post,  err := orm.Query[models.Post]().WithContext(ctx).Where("slug", "=", s).First()
count, err := orm.Query[models.Post]().WithContext(ctx).Count()
```

Operators: `=`, `!=`, `<`, `<=`, `>`, `>=`, `LIKE`, plus `WhereIn`.

All values are passed as `$N` placeholders — never string-concatenated. Column and direction names are validated against the model's real columns, so a typo is an error rather than an injection point.

### Pagination

```go
page, err := orm.Query[models.Post]().
    WithContext(ctx).
    OrderBy("id", "DESC").
    Paginate(pageNum, perPage)
```

```json
{ "data": [...], "page": 1, "per_page": 25, "total": 137, "total_pages": 6 }
```

`per_page` is clamped to `MaxPerPage` (100), so `?per_page=999999` can't turn one request into a table scan. An unpaginated `Get()` is additionally capped at `HardRowCap` (10,000 rows) and returns `orm.ErrTooManyRows` rather than silently truncating.

### Transactions

```go
err := orm.WithTransactionCtx(ctx, func(ctx context.Context, tx *sql.Tx) error {
    if err := orm.CreateCtx(ctx, &order, tx); err != nil {
        return err
    }
    return orm.CreateCtx(ctx, &payment, tx)     // any error rolls back
})
```

Panics inside the callback roll back too.

### Raw SQL

```go
exec, err := orm.DefaultExecutor()
rows, err := exec.QueryContext(ctx, `SELECT ... FROM ... WHERE x = $1`, x)
```

---

## 12. Relationships

Relationships are **always batched**. Each loader issues exactly one query no matter how many parents you pass — which is how you avoid the N+1 problem that dominates API latency.

```go
// hasMany — one query for all children
orm.LoadHasMany(ctx, users, "user_id",
    func(u *models.User) int64 { return u.ID },
    func(u *models.User, posts []*models.Post) { u.Posts = posts })

// belongsTo — one query for all parents
orm.LoadBelongsTo(ctx, posts,
    func(p *models.Post) int64 { return p.UserID },
    func(p *models.Post, u *models.User) { p.Author = u })

// hasOne
orm.LoadHasOne(ctx, users, "user_id",
    func(u *models.User) int64 { return u.ID },
    func(u *models.User, pr *models.Profile) { u.Profile = pr })

// belongsToMany, through a pivot table
orm.LoadBelongsToMany(ctx, posts, "post_tag", "post_id", "tag_id",
    func(p *models.Post) int64 { return p.ID },
    func(p *models.Post, tags []*models.Tag) { p.Tags = tags })
```

Typical use, in a service:

```go
func (s *PostService) List(ctx context.Context, page, perPage int) (*orm.Page[models.Post], error) {
    result, err := orm.Query[models.Post]().WithContext(ctx).Paginate(page, perPage)
    if err != nil {
        return nil, err
    }

    // 3 queries total, regardless of page size.
    if err := orm.LoadBelongsTo(ctx, result.Data,
        func(p *models.Post) int64 { return p.UserID },
        func(p *models.Post, u *models.User) { p.Author = u },
    ); err != nil {
        return nil, err
    }
    return result, nil
}
```

Loading is **explicit on purpose**. Lazy relations are how N+1 gets into a codebase invisibly; here every query is a line you can see.

> Relation fields must be tagged `db:"-"`, or the scanner treats them as columns and fails.

> `User.Posts` and `Post.Author` point at each other. Populating both directions on the same objects makes `json.Marshal` recurse forever — load one direction per request.

---

## 13. Migrations

```bash
gochin make migration create_posts_table --table=posts
gochin db migrate
gochin db migrate --rollback      # undo the most recent migration
```

Versions auto-increment (`000001`, `000002`, …) and are tracked in a `schema_migrations` table. Each migration runs in its own transaction.

### Schema builder

```go
// app/Migrations/000002_create_posts_table.go
func init() {
    orm.Register(orm.Migration{
        Version: "000002",
        Name:    "create_posts_table",
        Up: func(tx *sql.Tx) error {
            return orm.CreateTable(tx, "posts", func(t *orm.Blueprint) {
                t.ID()
                t.String("title")
                t.Text("body").Nullable()
                t.Boolean("published").Default("false")
                t.ForeignID("user_id").
                    References("users", "id").
                    OnDelete(orm.Cascade)
                t.Timestamps()
            })
        },
        Down: func(tx *sql.Tx) error {
            return orm.DropTable(tx, "posts")
        },
    })
}
```

**Columns:** `ID()`, `String()`, `Text()`, `Varchar(n)`, `Integer()`, `BigInteger()`, `Boolean()`, `Decimal(p,s)`, `Timestamp()`, `Date()`, `JSONB()`, `UUID()`, `ForeignID()`, `Timestamps()`

**Modifiers:** `.Nullable()`, `.Unique()`, `.Default(expr)`, `.Index()`, `.NoIndex()`, `.References(table, col)`, `.OnDelete(action)`, `.OnUpdate(action)`

**Actions:** `orm.Cascade`, `orm.Restrict`, `orm.SetNull`, `orm.NoAction`

**Table-level:** `t.Index("a","b")`, `t.UniqueIndex("a","b")`, `t.Raw(sql)`

**Other operations:** `orm.AlterTable`, `orm.DropTable`, `orm.DropColumn`

> **Foreign keys are indexed automatically.** Postgres indexes the *referenced* primary key, never the *referencing* column — without an index there, every cascade delete and join on it degrades to a sequential scan. Opt out with `.NoIndex()`.

Columns are `NOT NULL` by default. When adding one to a populated table, give it a `.Default(...)`.

---

## 14. Seeders

```bash
gochin make seeder Post
gochin db seed
gochin db seed --only=users
gochin db seed --list
```

```go
// app/Seeders/UserSeeder.go
func init() {
    orm.RegisterSeeder(orm.Seeder{
        Name:     "users",
        Priority: 10,              // lower runs first
        Run:      seedUsers,
    })
}

func seedUsers(ctx context.Context) error {
    existing, err := orm.Query[models.User]().
        WithContext(ctx).Where("email", "=", email).First()
    if err != nil && !errors.Is(err, orm.ErrRecordNotFound) {
        return err
    }
    if existing != nil {
        return nil                 // already seeded
    }

    hash, err := auth.Hash("password123456")
    if err != nil {
        return err
    }
    return orm.CreateCtx(ctx, &models.User{
        Name: "Ada Lovelace", Email: email, PasswordHash: hash,
    })
}
```

> **Seeders must be idempotent.** Nothing records which ones have run, so check before inserting — `gochin db seed` may be run any number of times.

The bundled seeder creates `ada@gochin.test` / `grace@gochin.test` / `alan@gochin.test`, all with password `password123456`, properly argon2-hashed so they can actually log in.

---

## 15. Authentication

Opaque, database-backed bearer tokens — revocable without extra machinery — with argon2id password hashing.

### Endpoints

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/api/v1/auth/register` | Create account → 201 + token |
| `POST` | `/api/v1/auth/login` | Exchange credentials → 200 + token |
| `GET` | `/api/v1/auth/me` | Current user |
| `POST` | `/api/v1/auth/logout` | Revoke this token → 204 |
| `POST` | `/api/v1/auth/logout-all` | Revoke every token for the user |

```bash
TOKEN=$(curl -s -X POST localhost:8080/api/v1/auth/login \
  -d '{"email":"ada@gochin.test","password":"password123456"}' | jq -r .token)

curl -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/auth/me
```

### Protecting routes

```go
v1.Get("/profile", ctrl.Profile, middleware.Auth())        // one route
users := v1.Group("/users", middleware.Auth())             // a whole group
v1.Get("/feed", ctrl.Feed, middleware.OptionalAuth())      // anonymous allowed
v1.Delete("/posts/{id}", ctrl.Destroy, middleware.Can("posts:delete"))
```

### Reading the current user

**In a controller:**

```go
func (c *PostController) Store(ctx *router.Context) error {
    user, err := services.MustAuthGuard().MustUser(ctx)
    if err != nil {
        return err
    }
    post := &models.Post{Title: req.Title, UserID: user.ID}
    ...
}
```

**In a service** — works with no signature change, because services already take a `context.Context`:

```go
func (s *PostService) CreateForCurrentUser(ctx context.Context, in requests.CreatePost) error {
    user, ok := services.MustAuthGuard().UserFromContext(ctx)
    if !ok {
        return router.Unauthorizedf("authentication required")
    }
    ...
}
```

The guard writes the authenticated state into both the request `Context` and its `context.Context`, which is what makes both call sites work.

Token metadata (id, abilities, expiry) is available via `Principal(ctx)` / `PrincipalFromContext(ctx)`.

### Password hashing

```go
hash, err := auth.Hash(plaintext)
ok, needsRehash := auth.Verify(hash, plaintext)
```

argon2id at OWASP's recommended profile, stored in standard PHC format:

```
$argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>
```

Cost parameters are read back out of each hash, so raising them later still verifies old passwords — `needsRehash` tells you when to upgrade one on successful login.

> **Why 19 MiB and not 64 MiB:** memory is charged per concurrent hash. At 64 MiB, 100 simultaneous logins is 6.4 GiB resident. A semaphore (`GOCHIN_ARGON2_MAX_CONCURRENT`) caps concurrent hashing — that, not a low memory setting, is the real protection.

### Security properties

These are deliberate and tested; keep them in mind before changing the auth path.

- **No user enumeration.** Wrong password and unknown email return byte-identical bodies, and the unknown-email path performs an equivalent hash so timings match.
- **Login is rate limited** per IP (`GOCHIN_LOGIN_RATE_PER_MINUTE`, default 5) with `Retry-After`. The global limiter is off by default, so credential endpoints carry their own.
- **Tokens are stored as SHA-256 digests.** A database dump cannot be replayed against the API.
- **A database outage returns 503, not 401** — otherwise every client would discard a valid token during an incident.
- **Presenting a revoked token logs a WARN** with token id, user id and IP: it's a compromise signal.
- **Emails are normalized** (lowercased, trimmed) with a unique index on `lower(email)`, so `Ada@X.com` and `ada@x.com` are one account.
- **`PasswordHash` is tagged `json:"-"`.** Never remove that tag.
- **There is no `POST /users`** — accounts are created only through `/auth/register`, so a password is always hashed and set.

---

## 16. Middleware

### Built in

Applied globally in this order:

```
RequestID → Logger → Recover → CORS → RateLimit → Timeout → [group] → [route] → handler
```

| Middleware | Purpose |
|---|---|
| `RequestID(trustInbound)` | Correlation id on every log line and error body |
| `Logger(cfg)` | One structured line per request |
| `Recover(debug)` | Panic → JSON 500, server stays up |
| `CORS(cfg)` | Cross-origin headers and preflight |
| `RateLimit(cfg)` | Token bucket per client IP |
| `Timeout(d)` | Cancels the request context, so DB queries stop too |

> `Logger` sits *outside* `Recover` on purpose: `Recover` turns a panic into a normal error return, so the log records the real status. Reversed, the log would fire mid-panic with a bogus status.

### Writing your own

```go
// app/Middleware/Timing.go
func Timing() router.Middleware {
    return func(next router.Handler) router.Handler {
        return func(c *router.Context) error {
            start := time.Now()
            err := next(c)
            logs.Debug("handled", "path", c.Request.URL.Path, "took", time.Since(start))
            return err
        }
    }
}
```

Return without calling `next` to short-circuit:

```go
if !allowed {
    return router.Forbiddenf("nope")     // handler never runs
}
```

```bash
gochin make middleware Timing
```

> Middleware chains are composed **once at registration**. Calling `Use()` after a group has registered routes panics, because those routes would silently miss it.

---

## 17. Logging

```go
import "github.com/sachinkaru123/gochin/pkg/logs"

logs.Debug("checkout started", "order_id", id)
logs.Info("user registered", "email", email)
logs.Warn("retrying", "attempt", n)
logs.Error("payment failed", "err", err)
```

### Inspecting a variable

```go
logs.Dump(order)                      // type + indented JSON + file:line
logs.Dump(user, "after update")       // with a label
```

```
level=DEBUG msg="after update" caller=UserService.go:42 type=*models.User
  value="{\n  \"id\": 7,\n  \"name\": \"Ada\"\n}"
```

### Channels

Route a subsystem to its own file:

```go
logs.Channel("payments").Info("captured", "amount", amount)
logs.DumpTo("payments", response)
```

→ `storage/logs/payments-2026-09-14.log`

### Bound context

```go
log := logs.With("user_id", user.ID, "request_id", c.RequestID())
log.Info("started")
log.Info("finished")
```

### Where logs go

`storage/logs/gochin-YYYY-MM-DD.log`, rotated daily by filename, plus stdout. Configure with `GOCHIN_LOG_DIR`, `GOCHIN_LOG_LEVEL`, `GOCHIN_LOG_FORMAT`, `GOCHIN_LOG_TO_FILE`, `GOCHIN_LOG_TO_STDOUT`.

Logging installs itself as `slog`'s default, so **anything using standard `log/slog` — including the router, auth guard and HTTP server — lands in the same files automatically.**

```bash
tail -f storage/logs/gochin-$(date +%F).log
```

> 4xx responses log at INFO, 5xx at ERROR. Client mistakes shouldn't page an operator.

---

## 18. File storage & uploads

```go
disk, err := storage.NewLocal("storage/app")

disk.Put(ctx, "reports/q1.pdf", reader)
rc, err := disk.Get(ctx, "reports/q1.pdf")
ok, err := disk.Exists(ctx, "reports/q1.pdf")
err = disk.Delete(ctx, "reports/q1.pdf")
```

`Disk` is an interface, so object storage can replace the local driver without touching call sites.

### Handling an upload

```go
func (c *MediaController) Upload(ctx *router.Context) error {
    file, err := ctx.FormFile("avatar")
    if err != nil {
        return err
    }

    if err := file.Validate(router.ImageRules(5 << 20)); err != nil {
        return err
    }

    path, err := file.Store(ctx.Ctx(), c.disk, "avatars")
    if err != nil {
        return err
    }
    return ctx.JSON(http.StatusCreated, map[string]string{"path": path})
}
```

Custom rules:

```go
rules := router.UploadRules{
    MaxBytes:          2 << 20,
    AllowedTypes:      []string{"application/pdf"},
    AllowedExtensions: []string{".pdf"},
}
```

Available metadata: `file.ClientName`, `file.Size`, `file.DeclaredType`, `file.SniffedType`.

### Upload security

- **The client filename is never used to build a path.** Files are stored under a random name with a validated extension, so `../../../etc/cron.d/evil.png` becomes `avatars/9f2c…d1.png`.
- **Types are checked by sniffing the content**, not the declared `Content-Type` — a PHP script renamed `.jpg` is rejected with 422.
- **Size is capped** before reading (`GOCHIN_MAX_UPLOAD_BYTES`), returning 413.
- **Uploads live in `storage/app`, which is never served statically**, so an uploaded `.html` or `.svg` can't run on your origin.

---

## 19. Static files

```go
r.Static("/assets", "public")     // GET /assets/app.css → public/app.css
```

Registered automatically from `GOCHIN_PUBLIC_DIR` / `GOCHIN_PUBLIC_URL`. Static routes bypass the API prefix.

Blocked by default:

| Request | Result |
|---|---|
| `/assets/.env`, `/assets/.git/config` | 404 — dotfiles |
| `/assets/` | 404 — no directory listings |
| `/assets/../../.env` | 404 — traversal |

Every response carries `X-Content-Type-Options: nosniff`.

---

## 20. Mail

```go
err := mail.Send(ctx, mail.Message{
    To:      []string{"ada@example.com"},
    Subject: "Welcome",
    Text:    "Thanks for signing up.",
    HTML:    "<h1>Thanks for signing up.</h1>",
})
```

Full message:

```go
mail.Message{
    From: "billing@example.com", FromName: "Billing",
    To:  []string{"a@example.com"},
    Cc:  []string{"b@example.com"},
    Bcc: []string{"c@example.com"},
    ReplyTo: "support@example.com",
    Subject: "Invoice",
    Text:    "See attached.",
    Attachments: []mail.Attachment{{
        Filename: "invoice.pdf", ContentType: "application/pdf", Content: pdf,
    }},
}
```

### Templates

```go
body, err := mail.Render("resources/views/emails/welcome.html", map[string]any{
    "Name": user.Name,
})
```

Rendered with `html/template`, so interpolated user data is escaped.

### Drivers

| `GOCHIN_MAIL_DRIVER` | Behaviour |
|---|---|
| `log` *(default)* | Writes the message to `storage/logs/mail-*.log` — **does not send** |
| `smtp` | Delivers over SMTP with STARTTLS |
| `array` | Captures in memory, for tests |

> The default is `log` on purpose: a misconfigured development machine must not be able to email real people. Switch to `smtp` explicitly.

Asserting in tests:

```go
box := mail.NewArrayMailer()
mail.SetDefault(box, "test@example.com", "Test")

// ... exercise code ...

if box.Count() != 1 {
    t.Fatalf("expected 1 email, got %d", box.Count())
}
```

---

## 21. Testing

```bash
go test ./pkg/...
go test -race ./pkg/...
go test -run TestUpload ./pkg/router/
```

Database-backed tests skip automatically when Postgres isn't reachable. To run them:

```bash
GOCHIN_DB_HOST=localhost GOCHIN_DB_USER=postgres \
GOCHIN_DB_PASSWORD=secret GOCHIN_DB_NAME=chingo_db \
go test ./pkg/...
```

Testing a handler needs no server:

```go
r := router.New()
r.Get("/users/{id}", ctrl.Show)
r.Build()

req := httptest.NewRequest(http.MethodGet, "/users/1", nil)
rec := httptest.NewRecorder()
r.ServeHTTP(rec, req)
```

---

## 22. Architecture notes

Worth knowing before you extend the framework.

**Dependency direction.** `app/Routes → app/Controllers → app/Services → pkg/orm`. Nothing in `pkg/` imports from an application's `app/` — ORM and Postgres driver errors are mapped to HTTP statuses inside the framework's own `pkg/bootstrap`, wired in automatically when your app starts, so your `app/` code never has to know about it. This is enforced in this repository's own CI:

```bash
go list -deps ./pkg/... | grep sachinkaru123/gochin/app && exit 1
```

**Registries over configuration.** Routes, migrations and seeders all register themselves from `init()` and are picked up by a single blank import. Adding a file never means editing a central list.

**Method values, not strings.** Laravel's `'UserController@show'` becomes `ctrl.Show` — a real function value. A typo is a compile error, not a runtime 500, and dispatch uses no reflection.

**Explicit dependency injection.** No container. Controllers are constructed in the route registrar, which is the composition root; a missing dependency fails the build.

**Where the time actually goes.** Routing costs ~0.5 µs while one Postgres round trip costs 200 µs–5 ms. Router micro-optimization is noise. What matters, in order: avoiding N+1 queries, having the right indexes, and sizing the connection pool.

---

## License

See [LICENSE](LICENSE).
