# Changelog (recent changes)

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

If you'd like this turned into a Git commit/PR branch structure and a real PR description, tell me the branch name to use and I can create the branch and prepare commit messages (I can't push the branch for you, but I can prepare it locally).