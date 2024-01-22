package shutdown

import (
	"context"
	"sync"
	"time"
)

var (
	defaultPipeline *Manager
	defaultLock     sync.Mutex
)

// Default returns the default manager. This method is thread-safe.
func Default() *Manager {
	if defaultPipeline == nil {
		defaultLock.Lock()
		defer defaultLock.Unlock()

		// if after acquiring the lock, pipeline is still nil, then set it.
		if defaultPipeline == nil {
			defaultPipeline = New()
		}
	}

	return defaultPipeline
}

// WaitForInterrupt blocks until an interrupt signal is received and all shutdown steps have been executed.
func WaitForInterrupt() {
	Default().WaitForInterrupt()
}

// Trigger starts the shutdown pipeline immediately. It will acquire a lock on the pipeline so all changes to the
// pipeline get blocked until the pipeline has completed. Panics and errors are all handled.
func Trigger(ctx context.Context) {
	Default().Trigger(ctx)
}

// AddParallelSequence is similar to AddSequence but it will execute the handlers all at the same time.
// AddParallelSequence(a) and AddParallelSequence(b) is not the same as AddParallelSequence(a, b). In the former, a
// runs and upon completion, b starts whereas in the latter case a and b both get started at the same time.
func AddParallelSequence(handlers ...NamedHandler) {
	Default().AddParallelSequence(handlers...)
}

// AddSequence adds sequencial steps meaning that these handlers will be executed one at a time and in the same order
// given.
// Calling AddSequence(a) and AddSequence(b) is same as AddSequence(a, b)
func AddSequence(handlers ...NamedHandler) {
	Default().AddSequence(handlers...)
}

// AddSteps adds parallel shutdown steps. These steps will be executed at the same time together or along
// with previously added steps if they are also able to run in parallel. In another word, calling AddSteps(a) and
// AddSteps(b) is same as AddSteps(a, b)
func AddSteps(handlers ...NamedHandler) {
	Default().AddSteps(handlers...)
}

// SetLogger sets the shutdown logger. If set to nil, no logs will be written.
func SetLogger(logger Logger) {
	Default().SetLogger(logger)
}

// SetShutdownFunc sets a function to get called after all of the shutdown steps have been executed. Regardless of
// panics or errors, this function will always get executed as the very last step. Even when a the pipeline times out,
// this function gets called before returning.
func SetShutdownFunc(f func()) {
	Default().SetShutdownFunc(f)
}

// SetTimeout sets the shutdown pipeline timeout. This indicates that when shutdown is triggered, the entire pipeline
// iteration must finish within the duration specified.
//
// NOTE: If the pipeline times out, the shutdown method is still called and some of the steps in the pipeline will
// still get scheduled but the blocking method (Trigger or WaitForInterrupt) will return immediately without waiting
// for the rest of the shutdown steps to complete.
func SetTimeout(duration time.Duration) {
	Default().SetTimeout(duration)
}
