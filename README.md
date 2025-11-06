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
- [2.1](https://github.com/fazstrac/devops-with-kubernetes/tree/2.1/log-output)
- [2.2](https://github.com/fazstrac/devops-with-kubernetes/tree/2.2/project)
- [2.3](https://github.com/fazstrac/devops-with-kubernetes/tree/2.3/log-output)
- [2.4](https://github.com/fazstrac/devops-with-kubernetes/tree/2.4/project)


## Directory structure (concise)

Only the most relevant folders and files are listed below. See each subfolder for full contents.

```
├── LICENSE
├── log-output/            # Two small log apps that demonstrate shared storage and readers
│   ├── app1/main.go       # writer: app1 writes timestamps to shared file
│   └── app2/main.go       # reader: app2 reads file and calls pong service
├── manifests/             # Cluster-level manifests (PV/PVC used in some exercises)
│   ├── log-pong-pv.yaml
│   └── log-pong-pvc.yaml
├── pong-app/              # Small counter service used by exercises
│   ├── main.go
│   └── Containerfile
├── project/               # Course project (todo frontend + todo-backend)
│   ├── manifests/
│   │   ├── ingress.yaml
│   │   ├── deploy-todo-app.yaml
│   │   └── deploy-todo-backend.yaml
│   ├── todo-app/          # Frontend (TypeScript + Go static server)
│   │   ├── main.go
│   │   ├── app.go
│   │   ├── ts/            # TypeScript sources and tests
│   │   └── templates/index.html
│   └── todo-backend/      # In-memory todo API (GET/POST /todos)
│       └── main.go
└── README.md              # This file
```

## Running tests

- The `project/todo-app` package contains integration and retry/backoff tests that intentionally wait. Run its tests with an extended timeout to avoid spurious failures:

	```bash
	cd project/todo-app
	go test -v ./... -timeout 2m
  npm run typecheck
  npm run test
  ```

- Quick per-module commands (from repo root):

	```bash
	cd project/todo-app && go test -v ./... -timeout 2m
  cd project/todo-backend && go test -v ./...
	cd pong-app && go test ./...
	cd log-output/app1 && go test ./...
	cd log-output/app2 && go test ./...
	```
