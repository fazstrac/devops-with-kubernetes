# Devops with Kubernetes - Log output app Exercise 2.3

## Purpose

At the moment the applications' purpose is to respond with a log of saved current date and a random UUID on `/log` endpoint + a count of hits on the `/pingpong` endpoint. UUID will be reset on restart, but logs and the number of pingpongs are persisted to files using emptyDir volumes. In this exercise this and Pong app were moved to namespace `exercises`

NB.
- EmptyDir volumes are removed if the pod is removed from the node (assuming also if they are evicted to another node)
- Concurrency on file access remains ignored at the moment. If needed, that should be handled using different means.

## Learning goals of the exercise as I understood them

* Kubernetes namespaces

## Notes

Pleasantly simple exercise, I was already using namespaces so this meant only changing the namespace.

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
│   ├── deploy.yaml     # Use this to deploy Log-output appliction
│   ├── ingress.yaml    # Defines the Ingress, copied from the pong-app -- should be refactored into one
│   └── service.yaml    # Defines Log-output app service
└── README.md           # This file
```
## See also

[Pong app](../pong-app) and [Exercise 2.1 in Chapter 3 / Networking Between Pods](http://courses.mooc.fi/org/uh-cs/courses/devops-with-kubernetes/chapter-3/networking-between-pods)
