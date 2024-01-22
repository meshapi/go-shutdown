// Package shutdown contains utilities to gracefully handle kernel interrupts and shutdown procedures.
//
// This package provides means to define a shutdown pipeline, which can contain sequential and parallel steps to
// complete a shutdown and also ways to trigger the pipeline either via signals or manually.
//
// To create a pipeline manager, use the following code:
//
//	shutdownManager := New()
//
// There is a default shutdown manager that can be accessed via Default() method and many package-level functions are
// available as a shortcut to accessing the default manager's methods. The default manager uses the standard log
// package for logging.
//
// The shutdown manager allows the addition of steps to the shutdown procedure, with some steps capable of running in
// parallel, while others run sequentially. The shutdown procedure can be triggered manually or by subscribing to
// kernel SIGTERM interrupt signals.
//
// There are three main methods for adding steps into the shutdown manager. The shutdown manager monitors the
// added steps and organizes them into sequential groups. Unless explicitly specified to form a distinct group,
// consecutive steps with the same parallel status are grouped together.
//
// * AddSteps: Adds steps that can run concurrently in separate goroutines.
//
//	Example:
//	In the example below, handlers 'a' and 'b' are called concurrently.
//
//	AddSteps(
//		HandlerFuncWithName("a", func(){}),
//		HandlerFuncWithName("b", func(){}))
//
// NOTE: Neighboring steps with the same parallelism status are grouped together. Thus the following lines have the
// same effect as the code above.
//
//	AddSteps(HandlerFuncWithName("a", func(){}))
//	AddSteps(HandlerFuncWithName("b", func(){}))
//
// * AddSequence: Adds steps that run one after another in the given order, without separate goroutines.
//
//	Example:
//	In the example below, handler 'a' is called first, followed by handler 'b'.
//
//	AddSequence(
//		HandlerFuncWithName("a", func(){}),
//		HandlerFuncWithName("b", func(){}))
//
// * AddParallelSequence: Adds steps to be executed in parallel, with an explicit instruction for the manager to wait
// for all steps prior to finish before starting the execution of this group.
//
//	Example:
//	In the example below handlers:
//		1. Handlers 'a' and 'b' are called concurrently.
//		2. After 'a' and 'b' finish, handlers 'c', 'd', and 'e' are called in parallel.
//		3. After 'c', 'd', and 'e' finish, handler 'e' is called, and upon completion, handler 'f' is called.
//
//	AddSteps(
//		HandlerFuncWithName("a", func(){}),
//		HandlerFuncWithName("b", func(){}))
//	AddParallelSequence(
//		HandlerFuncWithName("c", func(){}),
//		HandlerFuncWithName("d", func(){}))
//	AddSteps(HandlerFuncWithName("e", func(){}))
//	AddSequence(HandlerFuncWithName("f", func(){}))
//
// Additional features include:
// * SetLogger: Set a custom logger for the shutdown pipeline (default is plog.Default()).
// * SetTimeout: Define a timeout for the execution of the entire shutdown pipeline.
// * SetShutdownFunc: Add a callback function to handle the end of the shutdown pipeline, ensuring it gets called in
// any case, even in the presence of panics, errors, or timeouts.
package shutdown
