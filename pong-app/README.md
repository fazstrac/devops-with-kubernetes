# Devops with Kubernetes - Ping Pong app

## Purpose

At the moment its purpose is to respond with `pong <number>` to requests into its `/pingpong` endpoint. It maintains an internal counter on how many times it has been invoked. Counter is persisted into a file using PersistentVolumeClaim in [`../manifests/log-pong-pvc.yaml`](../manifests/log-pong-pvc.yaml).

Concurrency on file access is ignored at the moment. If needed, that should be handled using different means than accessing files directly. For instance, an obvious choice would be a Valkey/Redis counter.

I chose to hard code the `/pingpong` endpoint name instead of starting with url rewriting using Traefik. Anyway, as I understand, the ingresses need to adjusted to the enviroment where the applications are installed so doing rewriting didn't seem too rational choice to me.

## Files

```
pong-app
|
├── Containerfile       # Two-stage Docker build for building Go app and then
|                       # creating distroless lightweight container
├── go.mod              # Go module info
├── go.sum              # Go checksums, maintained by go mod tidy
├── main.go             # Main file
├── main_test.go        # Unit tests for the main file
├── manifests
│   ├── deploy.yaml     # Use this to deploy Pong appliction
│   └── service.yaml    # Defines the Pong app service
└── README.md           # This file
```

## See also

[Log output app](../log-output) and [Exercise 2.1 in Chapter 3 / Networking Between Pods](http://courses.mooc.fi/org/uh-cs/courses/devops-with-kubernetes/chapter-3/networking-between-pods)
