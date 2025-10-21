This repository contains small Go web services and Kubernetes manifests used for exercises in "DevOps with Kubernetes". The guidance below helps an AI coding assistant be immediately productive when editing, adding features, or fixing bugs in this codebase.

This repository is intentionally a learning-oriented course project for "DevOps with Kubernetes." The maintainer's primary goal is to learn production-minded best practices for Kubernetes, CI/CD, and service design while practising idiomatic Go (Golang) for backend services and JavaScript for the small frontend pieces. When making changes, prefer clear, teachable, and correct solutions that demonstrate realistic operational concerns (build args, container images, PVCs, sensible timeouts and retries). When proposing changes that affect production hardness (security, distributed locking, or storage migrations), explicitly call out tradeoffs and list follow-up steps the maintainer should take.

Learning goals & constraints
- Primary intent: this is a course project to learn DevOps with Kubernetes. Prioritize clarity and explainability over premature optimization.
- Languages: focus on idiomatic Go for backend services and minimal JavaScript for frontend examples. When you change code, prefer small, well-explained edits with tests and comments.
- Operational focus: include simple, realistic operational behavior (Containerfiles/ldflags, PVC usage, timeouts/retries). When proposing large or risky production changes (distributed locking, persistent stores, concurrent file locking), present two alternatives with tradeoffs and mark them as follow-up work.
- Learning preference: the maintainer prefers to try solving problems by themselves first; offer hints, references, and incremental guidance rather than full solutions on the first pass. If the maintainer explicitly asks for a full implementation, provide it, but label larger changes clearly as teaching examples vs production-ready.
- Tests & reproducibility: ensure tests remain runnable locally (document timeouts or env needed), and prefer small unit tests + one integration test over monolithic changes.

High-level architecture
- There are three main apps in top-level directories: `project`, `pong-app`, and `log-output`.
  - `project` is the course project (exercise 1.13). It is a Gin-based web server that serves an HTML index and a cached backend image. Key files: `project/app.go`, `project/main.go`, `project/templates/index.html`, `project/static/frontend.js`.
  - `pong-app` provides a simple `/pingpong` endpoint that returns `pong <n>` and persists a counter to a file. Key file: `pong-app/main.go`.
  - `log-output` contains two small apps that share a PVC to persist logs and counter files: `log-output/app1` (writes timestamp+UUID every 5s) and `log-output/app2` (reads the log and counter and exposes `/log`). Key files: `log-output/app1/main.go`, `log-output/app2/main.go`.

Design & important patterns
- Go + Gin is used across services. Routes are defined inline in `setupRouter` functions in each main.
- Persistence is intentionally simple: services write/read files on a shared PVC. There is no file locking. Treat this as a teaching exercise; do not attempt large concurrency refactors without noting intent.
- The `project` app implements an image caching and refresh protocol with these notable behaviors:
  - Cached image path is `./cache/image.jpg` by default (see `project/main.go`).
  - A background fetcher goroutine is controlled by `HeartbeatChan` and `StartPeriodicRefetchTrigger`. The fetcher sets `IsFetchingImageFromBackend` and uses `IsGracePeriodUsed` to allow one grace-serving of an expired image.
  - Fetch retries use a Fibonacci-style backoff and respect `Retry-After` headers when present (see `project/app.go`).

Build, run and test workflows (project-specific)
- Local Go build (fast): run from a service folder, e.g. `cd project && go build` or `go test ./...` to run tests for that module.
- Container image: each service has a `Containerfile` implementing a multi-stage build and injecting build-time vars `COMMIT_SHA` and `COMMIT_TAG`. Example (project): `docker build -f project/Containerfile --build-arg COMMIT_SHA=$(git rev-parse --short HEAD) -t my/project:dev project/`.
- Tests: `project` contains unit and integration tests (e.g. `integration_image_refresh_test.go`, `integration_concurrency_test.go`) which exercise the image refresh logic. Run only `go test` for the package to keep runs fast. Integration tests may assume filesystem state — run them in a clean workspace or adapt temp paths.
- Running tests (practical tips)
  - The `project` package contains retry/backoff and integration tests that intentionally wait; run its tests with a longer timeout to avoid spurious failures. Example (recommended):

    ```bash
    cd project
    go test -v ./... -timeout 2m
    ```

  - Quick per-module test commands from the repo root:

    ```bash
    cd project && go test -v ./... -timeout 2m
    cd pong-app && go test ./...
    cd log-output/app1 && go test ./...
    cd log-output/app2 && go test ./...
    ```

  - Optional Makefile snippet you can add to the repo to standardize local test runs:

    ```makefile
    .PHONY: test-all test-project test-pong test-log

    test-project:
	cd project && go test -v ./... -timeout 2m

    test-pong:
	cd pong-app && go test ./...

    test-log:
	cd log-output/app1 && go test ./... && cd - >/dev/null || true
	cd log-output/app2 && go test ./... && cd - >/dev/null || true

    test-all: test-project test-pong test-log
    ```

  - Tests produce verbose logs (Gin debug lines and custom logger output). This is expected; do not treat those as failures.
- Kubernetes manifests live under `manifests/` and per-app `manifests/` directories. PVC used for log/pong is `manifests/log-pong-pvc.yaml` (ReadWriteMany) — useful when running the two `log-output` containers and `pong-app` together.

Project conventions & common edits
- Entrypoints parse positional file arguments for filenames (e.g. `log-output/*` and `pong-app` expect filenames as argv). When editing API routes, preserve or note how filenames are passed by the container/manifest.
- Logging: apps use simple `fmt` or package-level `logger`. For `project`, `setupLogger()` returns a package-level `logger` used across app.go and main.go — prefer using that logger for consistency.
- Grace period and caching: the `project` app's cache logic is subtle — touching it requires respecting the mutex usage (`app.mutex`) and the boolean flags `IsFetchingImageFromBackend` and `IsGracePeriodUsed` to avoid racey behavior. When adding tests, prefer to exercise behaviors via exported methods (`StartBackgroundImageFetcher`, `LoadCachedImage`, `GetImage`) rather than manipulating internals.

Examples to reference when making changes
- Add a route to the `project` app: see `project/main.go` -> `setupRouter(app)` and `app.GetIndex`, `app.GetImage` handlers.
- Read/write counter file pattern: `pong-app/main.go` uses `initCounter` + `incrCounter` and writes the counter by truncating the file with `os.Create`. This is intentionally simplistic; keep similar style if editing other small examples.
- Background goroutine control: `project/app.go` `StartBackgroundImageFetcher` demonstrates how the code signals work via `HeartbeatChan`, `fetchResultChan`, and uses `context.Context` + a `WaitGroup` to manage lifecycle. Reuse this pattern for similar background workers.

Integration points and external dependencies
- External HTTP: `project` fetches images from an external URL (default `https://picsum.photos/1200`). Keep network timeouts (`FetchImageTimeout`) conservative in tests and unit mocks.
- Build-time injection: `COMMIT_SHA` and `COMMIT_TAG` are injected via `-ldflags` in the Containerfiles. When writing tests that assert on these variables, mock or set them in test setup.

What an AI assistant should do on common tasks
- Adding a small route: add handler in the service's `setupRouter` and add unit tests under that package. Use existing route patterns and response styles (plain text or JSON) and preserve status codes.
- Fixing a concurrency bug: first search for shared state protected by mutexes (`mutex`, `counterMutex`) and reproduce with a unit/integration test that isolates the race. In `project`, prefer using exported channels and methods to trigger behavior rather than directly manipulating state.
- Changing persistence format: if you change file formats, update both writer and reader services (`log-output/app1` writes logs; `log-output/app2` reads them). Keep tests in sync and add migration guidance in README when format changes.

Files to read first (quick jump list)
- `project/app.go`, `project/main.go`, `project/Containerfile`
- `pong-app/main.go`, `pong-app/Containerfile`
- `log-output/app1/main.go`, `log-output/app2/main.go`, their `Containerfile`s
- Top-level manifests: `manifests/log-pong-pv.yaml`, `manifests/log-pong-pvc.yaml`, per-app manifests under each `manifests/` subfolder

If something is ambiguous
- If the intent behind a change is unclear (for example, changing cache semantics or PVC access mode), propose two concrete alternatives and list the tradeoffs (backwards compatibility, test changes needed, Kubernetes manifest updates). Ask which behavior the maintainer prefers.

Short checklist to run locally when validating changes
1. Build the module: `cd <service> && go build` (or `go test ./...`).
2. Build image if necessary: `docker build -f <service>/Containerfile -t <name> <service>/` (pass COMMIT_SHA/COMMIT_TAG as build args).
3. If changing PVC-related behavior, apply manifests in minikube/kind and run both writer/reader pods together; verify files are visible across pods.

End of instructions — ask for clarifications or preferred style choices to iterate.
