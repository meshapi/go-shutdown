package shutdown

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Manager is a shutdown pipeline manager that can be configured to run through a number of parallel and sequencial
// steps when a shutdown is triggered. In order to start the shutdown procedure upon receiving a kernel Interrupt
// signal, use WaitForInterrupt() blocking method.
type Manager struct {
	steps            []shutdownStep
	timeout          time.Duration
	completionFuncnc func()
	logger           Logger
	lock             sync.Mutex
}

// New creates a new shutdown pipeline.
func New() *Manager {
	return &Manager{
		logger: NewStandardLogger(log.Default()),
	}
}

// SetTimeout sets the shutdown pipeline timeout. This indicates that when shutdown is triggered, the entire pipeline
// iteration must finish within the duration specified.
//
// NOTE: If the pipeline times out, the shutdown method is still called and some of the steps in the pipeline will
// still get scheduled but the blocking method (Trigger or WaitForInterrupt) will return immediately without waiting
// for the rest of the shutdown steps to complete.
func (m *Manager) SetTimeout(duration time.Duration) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.timeout = duration
}

// SetCompletionFunc sets a function to get called after all of the shutdown steps have been executed. Regardless of
// panics or errors, this function will always get executed as the very last step. Even when a the pipeline times out,
// this function gets called before returning.
func (m *Manager) SetCompletionFunc(f func()) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.completionFuncnc = f
}

// SetLogger sets the shutdown logger. If set to nil, no logs will be written.
func (m *Manager) SetLogger(logger Logger) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.logger = logger
}

// AddSteps adds parallel shutdown steps. These steps will be executed at the same time together or along
// with previously added steps if they are also able to run in parallel. In another word, calling AddSteps(a) and
// AddSteps(b) is same as AddSteps(a, b)
func (m *Manager) AddSteps(handlers ...NamedHandler) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if len(m.steps) == 0 {
		m.steps = append(m.steps, shutdownStep{handlers: handlers, parallel: true})
		return
	}

	lastStep := m.steps[len(m.steps)-1]
	if lastStep.parallel {
		lastStep.handlers = append(lastStep.handlers, handlers...)
		m.steps[len(m.steps)-1] = lastStep
		return
	}

	m.steps = append(m.steps, shutdownStep{handlers: handlers, parallel: true})
}

// AddSequence adds sequencial steps meaning that these handlers will be executed one at a time and in the same order
// given.
// Calling AddSequence(a) and AddSequence(b) is same as AddSequence(a, b)
func (m *Manager) AddSequence(handlers ...NamedHandler) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if len(m.steps) == 0 {
		m.steps = append(m.steps, shutdownStep{handlers: handlers, parallel: false})
		return
	}

	lastStep := m.steps[len(m.steps)-1]
	if !lastStep.parallel {
		lastStep.handlers = append(lastStep.handlers, handlers...)
		m.steps[len(m.steps)-1] = lastStep
		return
	}

	m.steps = append(m.steps, shutdownStep{handlers: handlers, parallel: false})
}

// AddParallelSequence is similar to AddSequence but it will execute the handlers all at the same time.
// AddParallelSequence(a) and AddParallelSequence(b) is not the same as AddParallelSequence(a, b). In the former, a
// runs and upon completion, b starts whereas in the latter case a and b both get started at the same time.
func (m *Manager) AddParallelSequence(handlers ...NamedHandler) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.steps = append(m.steps, shutdownStep{handlers: handlers, parallel: true})
}

// Trigger starts the shutdown pipeline immediately. It will acquire a lock on the pipeline so all changes to the
// pipeline get blocked until the pipeline has completed. Panics and errors are all handled.
func (m *Manager) Trigger(ctx context.Context) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if len(m.steps) == 0 {
		return
	}

	if m.timeout != 0 {
		newCtx, cancel := context.WithTimeout(ctx, m.timeout)
		ctx = newCtx
		defer cancel()
	}

	errorCount := 0
	resultChannel := make(chan handlerResult)

mainLoop:
	for _, step := range m.steps {
		remainingHandlers := len(step.handlers)

		go func() {
			for _, handler := range step.handlers {
				if step.parallel {
					go func(h NamedHandler) {
						m.executeHandler(ctx, h, resultChannel)
					}(handler)
				} else {
					m.executeHandler(ctx, handler, resultChannel)
				}
			}
		}()

		for remainingHandlers > 0 {
			select {
			case result := <-resultChannel:
				if result.Err != nil {
					errorCount++
					m.err(result.HandlerName + " shutdown failed: " + result.Err.Error())
				} else {
					m.info(result.HandlerName + " shutdown completed")
				}
				remainingHandlers--
			case <-ctx.Done():
				m.err("context canceled")
				errorCount++
				break mainLoop
			}
		}
	}

	if m.completionFuncnc != nil {
		m.completionFuncnc()
	}

	if errorCount > 0 {
		m.err(fmt.Sprintf("shutdown pipeline completed with %d errors", errorCount))
	} else {
		m.info("shutdown pipeline completed with no errors")
	}
}

func (m *Manager) info(text string) {
	if m.logger != nil {
		m.logger.Info(text)
	}
}

func (m *Manager) err(text string) {
	if m.logger != nil {
		m.logger.Error(text)
	}
}

func (m *Manager) executeHandler(ctx context.Context, handler NamedHandler, resultChannel chan<- handlerResult) {
	var err error

	defer func() {
		if panicErr := recover(); panicErr != nil {
			resultChannel <- handlerResult{HandlerName: handler.Name(), Err: fmt.Errorf("panic: %s", panicErr)}
		} else {
			resultChannel <- handlerResult{HandlerName: handler.Name(), Err: err}
		}
	}()

	err = handler.HandleShutdown(ctx)
}

// WaitForInterrupt blocks until an interrupt signal is received and all shutdown steps have been executed.
func (m *Manager) WaitForInterrupt() {
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
	<-exit

	m.info("received interrupt signal, starting shutdown procedures...")

	m.Trigger(context.Background())
}

type shutdownStep struct {
	handlers []NamedHandler
	parallel bool
}

type handlerResult struct {
	Err         error
	HandlerName string
}
