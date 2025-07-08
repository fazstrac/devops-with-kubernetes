# Devops with Kubernetes - Log output app

## Purpose

At the moment its purpose is to respond with current date and a random UUID on `/log` endpoint. UUID will reset on restart, no persistence.

## Files

```
log-output
|
├── Containerfile       # Two-stage Docker build for building Go app and then 
|                       # creating distroless lightweight container
├── go.mod              # Go module info
├── go.sum              # Go checksums, maintained by go mod tidy
├── main.go             # Main file
├── main_test.go        # Unit tests for main file 
├── manifests
│   ├── deploy.yaml     # Use this to deploy Log-output appliction
│   ├── ingress.yaml    # Defines the Ingress, here only for log-output app -- should be refactored
│   └── service.yaml    # Defines Pong app service
└── README.md           # This file
```

## See also

[Pong app](../pong-app) and [Exercise 1.9 in Chapter 2 / Introduction to networking](https://courses.mooc.fi/org/uh-cs/courses/devops-with-kubernetes/chapter-2/introduction-to-networking)
