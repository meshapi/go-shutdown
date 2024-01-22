package shutdown

import (
	"context"
	"fmt"
)

// Logger describes ability to write logs.
type Logger interface {
	// Info writes an information log.
	Info(text string)

	// Error writes an error log.
	Error(text string)
}

// Handler describes ability to handle a graceful shutdown.
type Handler interface {
	HandleShutdown(ctx context.Context) error
}

// NamedHandler is a handler that has a specific name.
type NamedHandler interface {
	Handler

	Name() string
}

type namedHandler struct {
	Handler
	name string
}

func (n namedHandler) Name() string {
	return n.name
}

// HandleFunc describes various different function signatures that can be used as a shutdown handler function.
type HandleFunc interface {
	func() | func() error | func(context.Context) | func(context.Context) error
}

// HandlerWithName returns a named handler with a specific name.
func HandlerWithName(name string, handler Handler) NamedHandler {
	return namedHandler{name: name, Handler: handler}
}

// HandlerFuncWithName returns a named handler for a function with a specific name.
//
// Accepted function signatures:
// func ()
// func (context.Context)
// func () error
// func (context.Context) error
func HandlerFuncWithName[H HandleFunc](name string, handleFunc H) NamedHandler {
	if handleFunc, ok := any(handleFunc).(func()); ok {
		return namedHandler{name: name, Handler: handlerFuncNoError(handleFunc)}
	}

	if handleFunc, ok := any(handleFunc).(func() error); ok {
		return namedHandler{name: name, Handler: handlerFunc(handleFunc)}
	}

	if handleFunc, ok := any(handleFunc).(func(context.Context)); ok {
		return namedHandler{name: name, Handler: handlerFuncContextNoError(handleFunc)}
	}

	if handleFunc, ok := any(handleFunc).(func(context.Context) error); ok {
		return namedHandler{name: name, Handler: handlerFuncContext(handleFunc)}
	}

	panic(fmt.Sprintf("unexpected function signature for handler: %T", handleFunc))
}

type (
	handlerFuncNoError        func()
	handlerFunc               func() error
	handlerFuncContext        func(context.Context) error
	handlerFuncContextNoError func(context.Context)
)

func (n handlerFuncNoError) HandleShutdown(context.Context) error {
	n()
	return nil
}

func (h handlerFunc) HandleShutdown(ctx context.Context) error {
	return h()
}

func (h handlerFuncContext) HandleShutdown(ctx context.Context) error {
	return h(ctx)
}

func (n handlerFuncContextNoError) HandleShutdown(ctx context.Context) error {
	n(ctx)
	return nil
}
