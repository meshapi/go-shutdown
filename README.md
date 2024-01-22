# Graceful shutdown utility for Go

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/meshapi/go-shutdown?style=flat-square)
![GitHub Tag](https://img.shields.io/github/v/tag/meshapi/go-shutdown)
![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/meshapi/go-shutdown/pr.yaml)
![GoReportCard](https://goreportcard.com/badge/github.com/meshapi/go-shutdown)
![GitHub License](https://img.shields.io/github/license/meshapi/go-shutdown?style=flat-square&color=blue)

The go-shutdown package offers a collection of utilities designed for managing shutdown procedures,
primarily focused on gracefully handling termination signals from the kernel, which is a common and
expected use case.

Ensuring graceful shutdowns is crucial for services to support zero-downtime deployments among other reasons.
During a deployment, the application that is currently running receives a SIGTERM signal and the process
can gracefully finish the work in progress and stop taking new requests in order to ensure no active task is
interrupted.

While this may seem straightforward, it is often overlooked when starting a new service or project.
The purpose of this package is to make it as easy as writing 2-3 lines of code to handle shutdowns
gracefully and also to remain flexible for more advanced usecases.


## Features

* [Use different concurrency options for steps in the shutdown pipeline](#configure-your-shutdown-pipeline)
* [Manual trigger or wait for SIGTERM signal](#shutdown-trigger)
* [Set timeout](#timeout)

## Installation

```bash
go get github.com/meshapi/go-shutdown
```

## Getting started

The way to handle shutdowns is to create and configure a shutdown pipeline and decide how it should get triggered.
There is a default shutdown pipeline for convenience but you can create separate pipelines as well.
Configuring the pipeline involves defining the steps and their concurrency. Finally deciding on triggering the pipeline
manually or subscribing to process signals.

### Configure your shutdown pipeline

To create a new shutdown pipeline use `shutdown.New` or simply use the package level methods to configure the default
shutdown pipeline.

There are three different methods to add new steps to the shutdown procedure, each with distinct concurrency behaviors.
Each has the same method signature which takes `shutdown.NamedHandler` instances. For logging purposes, each
handler/step must have a name.

Shortcut method `shutdown.HandlerWithName(string,Handler)` creates a `shutdown.NamedHandler` from a `shutdown.Handler`.

Shortcut method `shutdown.HandlerFuncWithName(string,HandleFunc)` creates a `shutdown.NamedHandler` from a callback and
accepts any of the following function types:
- `func()`
- `func() error`
- `func(context.Context)`
- `func(context.Context) error`

As new steps are added to the shutdown pipeline, the pipeline organizes the steps into sequential groups.
Unless explicitly specified to form a distinct group, consecutive steps with the same parallel status
are grouped together.

* `AddSteps`: Adds steps that can run concurrently in separate goroutines.

Example: In the example below, handlers 'a' and 'b' are called concurrently.

```go
shutdown.AddSteps(
    shutdown.HandlerFuncWithName("a", func(){}),
    shutdown.HandlerFuncWithName("b", func(){}))
```

> NOTE: Neighboring steps with the same parallelism status are grouped together. Thus the following lines have the
same effect as the code above.

```go
shutdown.AddSteps(shutdown.HandlerFuncWithName("a", func(){}))
shutdown.AddSteps(shutdown.HandlerFuncWithName("b", func(){}))
```

* `AddSequence`: Adds steps that run one after another in the given order, without separate goroutines.

Example: In the example below, handler 'a' is called first, followed by handler 'b'.

```go
shutdown.AddSequence(
    shutdown.HandlerFuncWithName("a", func(){}),
    shutdown.HandlerFuncWithName("b", func(){}))
```

* `AddParallelSequence`: Adds steps to be executed in parallel, with an explicit instruction for the manager to wait
for all steps prior to finish before starting the execution of this group.

Example: With the configuration code below:
1. Handlers 'a' and 'b' are called concurrently.
2. After 'a' and 'b' finish, handlers 'c', 'd', and 'e' are called in parallel.
3. After 'c', 'd', and 'e' finish, handler 'e' is called, and upon completion, handler 'f' is called.

```go
manager := shutdown.New()

manager.AddSteps(
    shutdown.HandlerFuncWithName("a", func(){}),
    shutdown.HandlerFuncWithName("b", func(){})).
manager.AddParallelSequence(
    shutdown.HandlerFuncWithName("c", func(){}),
    shutdown.HandlerFuncWithName("d", func(){})).
manager.AddSteps(shutdown.HandlerFuncWithName("e", func(){})).
manager.AddSequence(shutdown.HandlerFuncWithName("f", func(){}))
```

> NOTE: Many servers have a graceful shutdown method and they can be used with
> `shutdown.HandlerFuncWithName(string,HandleFunc)` method. The code below is an example of an HTTP server shutdown:

```go
httpServer := &http.Server{}
go func() {
    if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("HTTP server failed: %s", err)
    }
}()

shutdown.AddSteps(shutdown.HandlerFuncWithName("http", httpServer.Shutdown))
shutdown.WaitForInterrupt()
```

### Shutdown trigger

To manually trigger a shutdown, use `Trigger(context.Context)` method on a shutdown manager instance.
`shutdown.Trigger` is a shortcut method to trigger the default pipeline's shutdown procedure.

To trigger the shutdown when a SIGTERM is received, simply call `WaitForInterrupt()` method. This method blocks until
SIGTERM signal is received and the shutdown pipeline has concluded or deadline reached.

```go
manager := shutdown.New()

manager.Trigger(context.Background()) // manual trigger.
manager.WaitForInterrupt()            // block until SIGTERM is received and shutdown procedures have finished.
```

### Logging

In order to remain flexible with your logging tool, no choice over the logger is made here, instead any type that has
`Info` and `Error` methods, can be used as a logger.

The default shutdown pipeline uses the `log` package from the standard library but when you create new instances, no
logger is set and when no logger is available, no logging will be made.

Use the `SetLogger(Logger)` method to set a logger.

```go
var myLogger shutdown.Logger

manager := shutdown.New()
manager.SetLogger(myLogger)  // set the logger on the newly created shutdown pipeline.

shutdown.SetLogger(myLogger) // update the logger on the default pipeline.
```

### Timeout

Use method `SetTimeout(time.Duration)` to set a deadline on the shutdown pipeline's completion time. By default this is
not specified.

```go
shutdown.SetTimeout(15*time.Second)
```

### Completion callback

A completion function callback can be set via `SetCompletionFunc`. This function will get called in any case, even if
there are panics, errors or timeouts.

----------------

## Contributions

Contributions are absolutely welcome and please write tests for the new functionality added/modified.
Additionally, we ask that you include a test function reproducing the issue raised to save everyone's time.
