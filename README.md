# Devops with Kubernetes

## Exercises

- [1.1](https://github.com/fazstrac/devops-with-kubernetes/tree/1.1/log_output)
- [1.2](https://github.com/fazstrac/devops-with-kubernetes/tree/1.2/project)
- [1.3](https://github.com/fazstrac/devops-with-kubernetes/tree/1.3/log_output)
- [1.4](https://github.com/fazstrac/devops-with-kubernetes/tree/1.4/project)
- [1.5](https://github.com/fazstrac/devops-with-kubernetes/tree/1.5/project)
- [1.6](https://github.com/fazstrac/devops-with-kubernetes/tree/1.6/project)
- [1.7](https://github.com/fazstrac/devops-with-kubernetes/tree/1.7/log_output)
- [1.8](https://github.com/fazstrac/devops-with-kubernetes/tree/1.8/project)
- [1.9](https://github.com/fazstrac/devops-with-kubernetes/tree/1.9/pong-app)
- [1.10](https://github.com/fazstrac/devops-with-kubernetes/tree/1.10/log-output)
- [1.11](https://github.com/fazstrac/devops-with-kubernetes/tree/1.11/log-output)
- [1.12](https://github.com/fazstrac/devops-with-kubernetes/tree/1.12/project)
- [1.13](https://github.com/fazstrac/devops-with-kubernetes/tree/1.13/project)

## Directory structure

High-level view:
```
├── LICENSE
├── log-output  # Log application lies here
├── manifests   # General manifests pertaining to both Log application and Pong-app
├── pong-app    # Pong-app here
├── project     # The course project
└── README.md   # This files
```

## Running tests

- The `project` package contains integration and retry/backoff tests that intentionally wait. Run its tests with an extended timeout to avoid spurious failures:

	```bash
	cd project
	go test -v ./... -timeout 2m
	```

- Quick per-module commands (from repo root):

	```bash
	cd project && go test -v ./... -timeout 2m
	cd pong-app && go test ./...
	cd log-output/app1 && go test ./...
	cd log-output/app2 && go test ./...
	```

- Optional Makefile snippet to standardize test runs (add to repo root as `Makefile`):

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
