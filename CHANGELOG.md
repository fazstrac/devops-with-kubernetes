# Changelog (recent changes)

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

