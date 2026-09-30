# Project A — URL Shortener Roadmap

Scope: `urlshort/` only. Project B (`monitoring_system/`) runs in parallel
on its own track and does not wait on any milestone here.

## Environment

- Go 1.26.5, arm64, macOS
- Postgres 16, Redis, Prometheus, Grafana — all Homebrew, all local
- **No Docker Desktop.** `fly deploy` uses Fly's remote builder
- Deploy target: Fly.io
- Load testing: k6, run locally
- `NOTES.md`, `STATE.md`, `DESIGN.md`, `ROADMAP.md` live at the monorepo
  root and are committed to a **public** repo — no credentials,
  connection strings, or real hostnames in any of them

## Ground rules

- Predict before reading. Write the prediction in `NOTES.md` before
  looking anything up or reading a milestone's acceptance criteria.
- One milestone per commit. `STATE.md` updated in the same commit.
- Every design choice with more than one defensible answer gets a line
  in `DESIGN.md` with the reason, not just the decision.
- Don't start the next milestone until the current one's acceptance
  criteria actually pass. Not "should pass" — observed passing.
- If a milestone takes more than two sessions, split it.

---

# Phase 1 — HTTP foundation (in-memory)

Everything in this phase runs with no database and no external service.
The whole phase should be testable with `go test ./...` and `curl`.

## M1 — Server boot, timeouts, graceful shutdown, logging

**Goal:** A server that starts safely and stops cleanly. Timeouts and
shutdown belong at construction, not bolted on later.

**Action:** Build `urlshort/main.go` with `http.NewServeMux`, a
`/healthz` stub, and an explicit `http.Server` struct setting
`ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`.
Serve in a goroutine; block on `SIGINT`/`SIGTERM`; shut down with a
bounded context. Set up `log/slog` with a JSON handler now — every
later milestone logs through it.

**Acceptance:**
- `curl -i localhost:8080/healthz` returns 200
- Ctrl+C prints shutdown-start and shutdown-complete logs, no fatal
- The goroutine's error check ignores `http.ErrServerClosed` via
  `errors.Is` and exits on anything else
- Logs are structured JSON

**Predict:** What does `ListenAndServe` return when `Shutdown` is
called, and what breaks if you treat that as a fatal error? Does
`ReadTimeout` cancel `r.Context()` inside a slow handler?

## M2 — Store interface and in-memory implementation

**Goal:** Decouple handlers from storage before Postgres exists, so the
swap later is a constructor change and the tests don't move.

**Action:** Define a `LinkStore` interface — `Save(ctx, code, url)` and
`Get(ctx, code)`. Take `context.Context` from day one even though the
in-memory implementation ignores it. Implement with a map guarded by
`sync.RWMutex`.

**Acceptance:**
- `go test -race ./...` passes with a test hammering the store from
  multiple goroutines
- Handlers depend on the interface, never the concrete map

**Predict:** Why does a read need `RLock()` rather than no lock at all?
Why put `ctx` in the signature when nothing uses it yet?

## M3 — Code generation strategy

**Goal:** Decide how short codes are made. This decides the Postgres
schema, so it happens before the schema exists.

**Action:** Choose one and write the reasoning in `DESIGN.md`:
- Base62 of an incrementing counter — never collides, but every link is
  enumerable by walking the sequence
- Random codes — unguessable, but need collision detection and retry

**Acceptance:**
- Generator produces codes of a documented length and alphabet
- `DESIGN.md` records the choice, the trade-off, and what it implies
  for the schema
- If random: collision path is implemented and tested

**Predict:** With your chosen code length and alphabet, how many links
before collisions get likely? Does an enumerable scheme matter for this
project, and why or why not?

## M4 — Route skeleton

**Goal:** Wire up routing before filling in handler bodies.

**Action:** Register `POST /api/links` and `GET /{code}` on the
`ServeMux` with stub handlers. Extract the code with
`r.PathValue("code")`.

**Acceptance:**
- Each route reaches its own handler
- `/healthz` still resolves and isn't swallowed by `GET /{code}`
- Wrong method on a registered path returns 405

**Predict:** `/healthz` and `/{code}` both match a request to
`/healthz`. Which pattern wins, and by what rule?

## M5 — Body limits, JSON decoding, URL validation

**Goal:** Ingest client input safely. This is where open redirect is
prevented — not in a review at the end.

**Action:** Wrap `r.Body` in `http.MaxBytesReader` with a limit of a few
KB. Decode JSON into a struct. Validate that the target URL parses, that
the scheme is `http` or `https`, and that `u.Host != ""`.

**Acceptance:**
- Oversized body returns 413; detected with
  `errors.As(err, &*http.MaxBytesError)`, not a string match
- `javascript:alert(1)` returns 400
- `http:foo` and `https:///` return 400
- Malformed JSON returns 400, not 500

**Predict:** Why isn't a scheme check alone enough? What does
`url.Parse` accept that you'd expect it to reject?

## M6 — Create and redirect against the in-memory store

**Goal:** First end-to-end behavior.

**Action:** Create stores a generated code and returns 201 with the
code. Redirect looks up the code and issues a redirect with a `Location`
header, or 404.

**Acceptance:**
- POST returns 201 and a code; the link is in the store
- GET on that code redirects; `curl -v` shows status and `Location`
- Unknown code returns 404
- `DESIGN.md` records which redirect status you chose and why

**Predict:** What happens to a user if you pick 301 and later need to
change where a code points?

## M7 — Handler tests

**Goal:** Lock in Phase 1 behavior before persistence can hide
regressions.

**Action:** Table-driven tests using `httptest`, against the in-memory
store. Cover: valid create, missing URL, malformed JSON, rejected
scheme, hostless URL, oversized body, known-code redirect, unknown-code
404.

**Acceptance:**
- `go test -race ./...` passes
- No test needs Postgres or Redis
- Each test builds its own store; no shared state between tests

**Predict:** Which of these are you most likely to have gotten wrong?
Write the guess before running them.

---

# Phase 2 — Persistence

## M8 — Config, Postgres pool, migrations

**Goal:** Make the app configurable and connect to the database. No
behavior change yet.

**Action:** Read port and database URL from environment variables —
port stops being hardcoded here, not on deploy day. Initialize a
`pgxpool` at boot with a boot-time context, not a request context. Write
the schema to match M3, applied by `schema.sql` or `goose`. Extend the
M1 shutdown path to close the pool.

**Acceptance:**
- App connects to local Homebrew Postgres
- Schema applies from a committed file or migration tool, repeatably
- `.env` is in `.gitignore` and no connection string is in any tracked
  file
- Ctrl+C closes the pool before exiting

**Predict:** Why can't the pool be built from `r.Context()`? What
happens to in-flight queries if you exit without closing the pool?

## M9 — Persistent creation with bounded queries

**Goal:** Links survive a restart, and a slow database can't pin a
request open.

**Action:** Postgres-backed `Save`. Wrap the query in
`context.WithTimeout(r.Context(), ...)`.

**Acceptance:**
- Created links survive a server restart
- A database error returns 500 with a structured log, never a panic
- If Postgres is stopped mid-request, the request fails on the context
  deadline rather than hanging
- The unique constraint is exercised (M3's collision path, if random)

**Predict:** `WriteTimeout` already exists on the server. Why doesn't
it bound the database query?

## M10 — Persistent lookup and readiness

**Goal:** Redirects read from Postgres. Health reflects reality.

**Action:** Postgres-backed `Get` using the request context. Keep
`/healthz` as a liveness check that only proves the process is up, and
add `/readyz` that pings the database — or record in `DESIGN.md` that
you're deliberately not doing readiness and why.

**Acceptance:**
- Redirects work from Postgres after a restart
- In-memory store is out of the production path but still present and
  still used by M7's tests
- With Postgres stopped, `/healthz` passes and `/readyz` fails

**Predict:** If Fly health-checks `/healthz` and Postgres is down, what
does Fly think about your app? Is that what you want?

---

# Phase 3 — Observability and caching

## M11 — Prometheus client and RED metrics

**Goal:** Expose metrics. This must come before any cache counters —
the client has to exist before anything can register against it.

**Action:** `promhttp` at `/metrics`. Add request rate, error count, and
duration histogram. Labels: method, route pattern, status class.

**Acceptance:**
- Local Prometheus scrapes the endpoint and the target is healthy
- Traffic moves the counters
- No label contains a short code or a full path — bounded cardinality
  only

**Predict:** What happens to Prometheus if you label by short code?
Estimate the series count after a thousand links.

## M12 — Redis cache with TTL and negative caching

**Goal:** Keep repeat lookups off Postgres, including lookups for codes
that don't exist.

**Action:** Check Redis, fall back to Postgres, populate Redis on a hit
with a TTL. Cache a not-found marker with a shorter TTL. Close the Redis
client in the shutdown path.

**Acceptance:**
- Second request for the same code is served from Redis, verified via
  `redis-cli MONITOR` or a log line
- Repeated requests for an unknown code hit Postgres once, not every
  time
- Keys expire

**Predict:** Someone scans a thousand random codes against your service.
Without negative caching, what does Postgres see? With it?

## M13 — Cache metrics and failure fallback

**Goal:** Redis is an optimization, not a dependency.

**Action:** Counters for cache hit, miss, and error. Configure a short
Redis timeout. Verify fallback by stopping Redis.

**Acceptance:**
- `brew services stop redis` — redirects still work, latency rises,
  error counter climbs
- No hang, no crash
- Cache metrics visible in Prometheus

**Predict:** With no timeout configured, what does a request do when
Redis accepts the connection but never answers?

---

# Phase 4 — Security and production

## M14 — Rate limiting

**Goal:** An unauthenticated create endpoint on a public URL is a free
phishing-link generator. This lands before the deploy, not after.

**Action:** Per-IP limiting on `POST /api/links`. Token bucket in memory
or a Redis counter. Record the chosen limit and reasoning in
`DESIGN.md`.

**Acceptance:**
- Exceeding the limit returns 429
- The limiter is metered — a counter for rejected requests
- Redirects are not rate-limited, or are limited far more loosely

**Predict:** Where does the client IP actually come from once you're
behind Fly's proxy? Is `r.RemoteAddr` still meaningful?

## M15 — Log sanitization review

**Goal:** Target URLs carry password-reset tokens and signed storage
URLs. Once on Fly, logs are retained.

**Action:** Walk every log call site that touches a link. Decide per
site: drop the URL, log host only, or log the code instead. Record the
policy in `DESIGN.md`.

**Acceptance:**
- A written list in `DESIGN.md` of every site reviewed and the decision
  at each
- No full target URL reaches a log outside local development

**Predict:** Which log line would you have missed if you hadn't gone
looking deliberately?

## M16 — Fly provisioning and secrets

**Goal:** Real infrastructure before the first deploy. Secrets pointing
at nothing is not a deploy.

**Action:** Provision managed Postgres and Redis. Set `DATABASE_URL` and
`REDIS_URL` with `fly secrets set` — never in `fly.toml`, never in git.
Add `release_command` to `fly.toml` so migrations run before new code
starts.

**Acceptance:**
- Both services provisioned and reachable
- `fly secrets list` shows the names; no value appears in any tracked
  file
- `release_command` applies migrations

**Predict:** What happens if migrations run *after* the new version
starts serving instead of before?

## M17 — Dockerfile and deploy

**Goal:** Ship it.

**Action:** Multi-stage Dockerfile. Base the final stage on
`distroless/static` or alpine — **not `scratch`**, which has no CA
certificates and will fail TLS to managed Postgres and Redis. Deploy
with `fly deploy` using the remote builder; no local Docker needed.

**Acceptance:**
- Live on a `.fly.dev` host
- `/healthz` passes Fly's health check
- TLS to managed Postgres and Redis succeeds
- Creating and following a link works against the public URL

**Predict:** Why would a `scratch` image work fine locally and fail the
moment it talks to a managed database?

---

# Phase 5 — Load and the Project B payoff

## M18 — k6 load generation, locally

**Goal:** Sustained traffic for Project B's scraper to observe.

**Action:** k6 script mixing creates, known-code redirects, and unknown
codes. Run it **against the local instance**, not the Fly deployment.
Point Project B at the local `/metrics` while it runs.

Two reasons this stays local: M14's rate limiter would throttle k6 and
you'd be measuring the limiter instead of the app, and remote scraping
means exposing `/metrics` publicly — request volume, error rates, and
route names to anyone who curls it.

**Acceptance:**
- k6 sustains traffic without the rate limiter distorting the run
- Cache hit ratio, error rate, and latency all visibly move under load
- Project B scrapes the local endpoint throughout

**Predict:** Which metric will move first as you increase load, and
where do you expect the first bottleneck — Postgres, Redis, or the Go
process?

---

# Sequence

```text
Phase 1  M1 → M2 → M3 → M4 → M5 → M6 → M7    (in-memory, tested)
Phase 2  M8 → M9 → M10                       (Postgres, context-bounded)
Phase 3  M11 → M12 → M13                     (metrics, then cache)
Phase 4  M14 → M15 → M16 → M17               (safe before public)
Phase 5  M18                                 (load, local)
```

Hard ordering constraints, if you ever reshuffle:

- M2 before M9 — the interface has to exist before the swap
- M3 before M8 — code strategy decides the schema
- M4 before M5 — a handler has to exist before you can fill it
- M11 before M13 — no counters before the client
- M14 and M15 before M17 — nothing public without limits and clean logs

# Model routing

- Gemini: syntax, standard library lookup, most of Phase 1
- Claude: M3 (code strategy), M9 (context semantics), M12–M13 (cache
  correctness), M14–M15 (security review)
- The pattern: escalate where a mistake is silent. Anything that fails
  loudly on the first run doesn't need a strong model.
