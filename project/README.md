# Devops with Kubernetes - Project app

## Purpose

At the moment its purpose is to respond with build information and a greeting on the root endpoint.

## Files

```
project
├── Containerfile       # Two-stage Docker build for building Go app and then
|                       # creating distroless lightweight container 
├── go.mod              # Go module info
├── go.sum              # Go checksums, maintained by go mod tidy
├── main.go             # Main file
├── main_test.go        # Unit tests for main file
├── manifests
│   ├── deploy.yaml     # Use this to deploy the Project application
│   ├── ingress.yaml    # Defines the Ingress for the application, currently responds to hostname
|   |                   # project.fudwin.xyz
│   └── service.yaml    # Defines the service to connect ingress and pods
├── README.md           # This file
└── templates
    └── index.html      # Template HTML file for index endpoint
```

## See also

[Pong app](../pong-app) and [Log output](../log-output/)