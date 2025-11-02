# Changelog (recent changes)

## 2025-11-02 — exercise/2p2 — project: UI polish, TS/tests, CI stabilisation and backend API

Summary:
- Add playful CSS and improve mobile friendliness for the todo UI; convert the index to a responsive, table-based layout and update the frontend population code (`project/todo-app/templates/index.html`, `project/todo-app/static/styles.css`, `project/todo-app/static/frontend.js`).
- Introduce a TypeScript frontend (`project/ts/*`) with initial unit tests; switch the JS test runner to `vitest`/yarn, fix missing imports and other TS test errors.
- Reorganise folders to accommodate frontend and `todo-backend` services, add/update `package.json` and `vitest` config to support the new TS tests.
- Add new endpoint `/todos` (located in `todo-backend`), which supports HTTP verbs GET, POST, PATCH, and DELETE to handle dynamic todos.
- Improve CI: remove explicit Go/Node setup steps in the workflow to rely on runner images, correct artifact/coverage collection paths, and iterate on CI fixes to stabilise JS+Go coverage reporting.
- Misc: remove placeholder content from the UI, baseline TypeScript compilation, and several small commits to stabilise builds and tests.

How to verify:
- Run frontend unit tests: `cd project && yarn && yarn test` (or use the JS test runner configured in the repo).
- Run Go tests for the project package: `cd project && go test -v ./... -timeout 2m`.
- Manually exercise the UI/backend locally (see `project/README.md` for run instructions) and verify PATCH/DELETE behaviour against `todo-backend`.

Files changed (not exhaustive):
- `project/todo-app/templates/index.html`, `project/todo-app/static/styles.css`, `project/todo-app/static/frontend.js`, `project/ts/main.ts`, `project/ts/todo-api.ts`, `project/ts/tests/*`, `project/package.json`, `project/vitest.config.ts`, `project/todo-backend/main.go`, CI workflow and artifact/coverage scripts, assorted test fixes and README updates.

## 2025-10-26 — exercise/2p1 — log-output & pong-app: app2 integration, tests and manifests

Summary:
- Add a lightweight log-reader (`log-output/app2`) that reads a shared log file and augments it with the current pong counter by calling the `pong-app` service.
- Improve test ergonomics by using httptest mocks and silencing Gin logs in tests.
- Update container images and Kubernetes deployment manifests for the exercise (`dev-exercise-2p1` tags).
- Remove PV/PVC (top-level manifests) in favor of an emptyDir-backed shared volume in the per-app deployment.

Rationale / Why:
- Introduces `log-output/app2` as a companion consumer that demonstrates cross-pod communication via a shared filesystem plus a service-to-service HTTP call to display combined information (teaching exercise).
- Tests were improved to be deterministic and quieter by mocking network calls and silencing Gin test logs.
- Manifests were simplified for the exercise to use an ephemeral shared volume (`emptyDir`), making it easier to run locally (Minikube/Kind) without provisioning a ReadWriteMany PVC.

Minimal file changes (focus):
- log-output/app2:
	- `main.go` — reads log file, calls `PONGAPP_SVC_URL`, serves `/log`.
	- `main_test.go` — tests with `httptest.Server` and temp file.
	- `Containerfile` — multi-stage build; distroless final.
- log-output/manifests/deploy.yaml — add `log-output-app2`, set `PONGAPP_SVC_URL`, use `emptyDir`, image tags `dev-exercise-2p1`.
- pong-app:
	- `main.go` — require one filename arg, expose `/pingpong` and `/pongs`.
	- `main_test.go` — router tests using `t.TempDir()`.
	- `manifests/deploy.yaml` — image tag updated to `dev-exercise-2p1`.
- Top-level manifests:
	- Removed PV/PVC (`manifests/log-pong-pv.yaml`, `manifests/log-pong-pvc.yaml`); added placeholder `manifests/.keep`.

How to verify:
- Run tests:
	- `cd pong-app && go test ./...`
	- `cd log-output/app2 && go test ./...`
- Run `log-output/app2` locally (example):
	```bash
	export PONGAPP_SVC_URL="http://localhost:4000/pongs"
	mkdir -p /tmp/logdir
	echo "Hello logs" > /tmp/logdir/log.txt
	cd log-output/app2
	go run . /tmp/logdir/log.txt
	# visit http://localhost:8080/log
	```

## 2025-10-21 — Repo housekeeping and test ergonomics

Summary:
- Added `.github/copilot-instructions.md` with repository-specific guidance for AI coding assistants and contributors.
- Updated top-level `README.md` with a "Running tests" section documenting extended timeout for `project` tests.
- Reduced noisy test output for the `project` package by setting Gin to release mode and silencing the package logger in test setup (`project/setup_test.go`).
- Created a temporary `Makefile` to standardize running tests across modules, then removed it at your request.

Why:
- `project` contains integration tests that intentionally wait and use retry/backoff logic; the default `go test` timeout can cause spurious failures. The README and instructions were added to help contributors run tests reliably.
- Silencing framework and package logs during tests makes CI and local runs easier to read and focuses on test results.

How to reproduce locally:

Run the `project` tests with the recommended timeout:

```bash
cd project
go test -v ./... -timeout 2m
```

Run all module tests manually (no Makefile):

```bash
cd project && go test -v ./... -timeout 2m
cd pong-app && go test ./...
cd log-output/app1 && go test ./...
cd log-output/app2 && go test ./...
```

Files changed in this update:
- `.github/copilot-instructions.md` — new, repo-specific AI assistant guidance.
- `README.md` — added test-running guidance and example Makefile snippet (for optional use).
- `project/setup_test.go` — set Gin to release mode and discard logger output during tests.
- `CHANGELOG.md` — this file.

