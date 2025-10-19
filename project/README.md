# Devops with Kubernetes - Project app Exercise 1.13

## Purpose

At the moment its purpose is to respond with build information, an image fetched from a backend server and a greeting on the root endpoint. The image is such that it cached for a given amount of time (10 min), after which it fetched automatically again, which a grace period during which while image image is being fetched, exactly once, as per the assignment.

## Learning goals of the exercise as I understood them

* Add a bit more of HTML into the project from ex. 1.12

## Extra learning goals I created for myself
* Learn Javascript

## Notes

Not a lot to take notes from. Maybe this could have been combined into another exercise? It took roughly 10 minutes including writing this text to implement this exercise.


## Files

```
project
├── manifests
│   ├── deploy.yaml                     # Use this to deploy the Project application
│   ├── ingress.yaml                    # Defines the Ingress for the application, currently 
|   |                                   # responds to hostname project.fudwin.xyz
│   ├── project-pv.yaml                 # Project persistent volume setup
│   ├── project-pvc.yaml                # Project persistent volume claim
│   └── service.yaml                    # Defines the service to connect ingress and pods
├── templates               
│   └── index.html                      # Template HTML file for index endpoint
├── static               
│   └── frontend.js                     # Quick and dirty Javascript to add the new TODO. Not persisted at 
|                                       # this point
├── Containerfile                       # Two-stage Docker build for building Go app and then
|                                       # creating distroless lightweight container 
├── README.md                           # This file
├── app.go                              # App struct definition and the program logic
├── go.mod                              # Go module info
├── go.sum                              # Go checksums, maintained by go mod tidy
├── integration_concurrency_test.go     # Integration tests for concurrent access - to verify
|                                       # correct usage of the grace period and a bit stress testing
├── integration_image_refresh_test.go   # Integration tests for image refresh logic
├── integration_startup_test.go         # Integration tests for correct startup behavior - reuse
|                                       # the cached image if it's present and available on startup
├── main.go                             # Main file
├── setup_test.go                       # Test setup + common utilities
└── unit_app_test.go                    # app.go unit tests

```


## See also

[Pong app](../pong-app) and [Log output](../log-output/)