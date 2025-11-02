# Devops with Kubernetes - Project app Exercise 2.2

## Purpose

Course project's purpose is to respond with build information, an image fetched from a backend server and a greeting on the root endpoint. The image is such that it is cached for a given amount of time (10 min), after which it fetched automatically again. A grace period exists during the image fetch such that the while image is being fetched, the old image can be returned to client exactly once, as per the assignment. This is running in the todo-app.

Todo-backend is a microservice that exposes endpoint `/todos` with HTTP verbs GET, POST, DELETE, and PATCH. A Javascript single-page app handles fetching, updating, deleting, and creating todos, using the `/todos` endpoint, but the UI supports only GET and POST for now. Also for now, the todos are not persisted.

## Learning goals of the exercise as I understood them

* Intercontainer networking - in this case routing endpoints to different backends
* Using Javascript to bring dynamic updates to the app
* Add a bit more of HTML into the project from ex. 1.12

## Extra learning goals I created for myself

* Learn Typescript, testing it and compiling into Javascript

## Notes

Fun exercise and quite straightforward. Learned a lot about Typescript and esp. testing it. It's quite rich ecosystem I see.


## Files

```
project
├── manifests
│   ├── deploy-todo-app.yaml            # Deployment manifest for the todo-app (frontend)
│   ├── deploy-todo-backend.yaml        # Deployment manifest for the todo-backend (API)
│   ├── ingress.yaml                    # Ingress for the application (host: project.fudwin.xyz)
│   ├── project-pv.yaml                 # Project persistent volume setup
│   ├── project-pvc.yaml                # Project persistent volume claim
│   ├── service-todo-app.yaml           # ClusterIP service for the todo-app
│   └── service-todo-backend.yaml       # ClusterIP service for the todo-backend
├── todo-app
│   ├── app.go                          # Frontend app logic (serves template + static)
│   ├── Containerfile                   # Frontend container build (TS -> JS + Go server)
│   ├── go.mod
│   ├── go.sum
│   ├── integration_concurrency_test.go # Integration tests for concurrent behaviour
│   ├── integration_image_refresh_test.go
│   ├── integration_startup_test.go
│   ├── main.go
│   ├── package.json
│   ├── setup_test.go
│   ├── templates
│   │   └── index.html                  # Template HTML file for index endpoint
│   ├── ts/
│   │   ├── main.ts                     # Frontend entry (builds to static JS)
│   │   ├── todo-api.ts                 # Thin typed wrapper for /todos API calls
│   │   ├── tsconfig.jest.json
│   │   ├── tsconfig.json
│   │   └── tests                       # Unit/DOM tests for the frontend
│   │       ├── main.dom.test.ts
│   │       └── todo.test.ts
│   ├── unit_app_test.go                # Frontend unit tests
│   ├── vitest.config.ts
│   ├── vitest.setup.ts
│   └── yarn.lock
├── todo-backend
│   ├── Containerfile                   # Backend container build
│   ├── go.mod
│   ├── go.sum
│   ├── main.go                         # Backend API (GET/POST /todos)
│   └── main_unit_test.go               # Backend unit tests
├── README.md                           # This file
└── go.mod                              # Go module info (workspace-level)

```


## See also

[Pong app](../pong-app) and [Log output](../log-output/)
