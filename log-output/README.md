# Devops with Kubernetes - Log output app

## Purpose

At the moment its purpose is to respond with a log of saved current date and a random UUID on `/log` endpoint. UUID will reset on restart, no persistence.

## Files

```
log-output
├── app1
│   ├── Containerfile   # Two-stage Docker build for building Go app and then
|   |                   # creating distroless lightweight container
│   ├── go.mod          # Go module info
│   ├── go.sum          # Go checksums, maintained by go mod tidy
│   ├── main.go         # Main file - this one appends the log text to data file every 5 seconds
│   └── main_test.go    # Unit tests for main file
├── app2
│   ├── Containerfile   # Two-stage Docker build for building Go app and then
|   |                   # creating distroless lightweight container
│   ├── go.mod          # Go module info
│   ├── go.sum          # Go checksums, maintained by go mod tidy
│   ├── main.go         # Main file - this one reads the data file and prints it out on the web endpoint
│   └── main_test.go    # Unit tests for main file
├── manifests
│   ├── deploy.yaml     # Use this to deploy Log-output appliction - this one creates an emptyDir volume
|   |                   # which is shared between the containers. Should be templatable for simpler switch
|   |                   # from my dev environment to "release" environment via Github.
│   ├── ingress.yaml    # Defines the Ingress, copied from the pong-app -- should be refactored into one
│   └── service.yaml    # Defines Log-output app service
└── README.md           # This file
```
## See also

[Pong app](../pong-app) and [Exercise 1.10 in Chapter 2 / Introduction to networking](https://courses.mooc.fi/org/uh-cs/courses/devops-with-kubernetes/chapter-2/introduction-to-networking)
