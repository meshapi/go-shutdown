package shutdown_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/meshapi/go-shutdown"
)

type LogWrapper struct {
	Writer io.Writer
}

func (l LogWrapper) Info(text string) {
	_, _ = l.Writer.Write([]byte(text))
}

func (l LogWrapper) Error(text string) {
	_, _ = l.Writer.Write([]byte(text))
}

// EventTime is to store at what relative time a process completes or should complete.
type EventTime map[string]time.Duration

func (e EventTime) String() string {
	keys := []string{}
	for key := range e {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	writer := &strings.Builder{}
	for _, key := range keys {
		_, _ = fmt.Fprintf(writer, "%s\t%d\n", key, e[key].Milliseconds())
	}
	return writer.String()
}

// MustParseEventTime parsed <name>:<duration>,... format.
func MustParseEventTime(value string) EventTime {
	parts := strings.Split(value, ",")
	result := EventTime{}
	for _, part := range parts {
		sections := strings.Split(part, ":")
		duration, err := time.ParseDuration(sections[1])
		if err != nil {
			panic("failed to parse time duration " + sections[1])
		}
		result[sections[0]] = duration
	}
	return result
}

// SequenceMonitor captures events and marks their time of execution and asserts if they occurred in a certain time
// frame.
type SequenceMonitor struct {
	events    EventTime
	startTime time.Time
	lock      sync.Mutex
}

// Mark stores the relative time of completion of the given procedure. Needs to be called after StartRecording()
func (s *SequenceMonitor) Mark(name string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.events[name] = time.Since(s.startTime)
}

// EventTime returns the current recorded event time.
func (s *SequenceMonitor) EventTime() EventTime {
	return s.events
}

// StartRecording sets the start time and initializes the event time.
func (s *SequenceMonitor) StartRecording() {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.events = EventTime{}
	s.startTime = time.Now()
}

// Matches returns whether or not the captured events and their timelines matches the input.
func (s *SequenceMonitor) Matches(input EventTime) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	if len(s.events) != len(input) {
		return false
	}

	for inputKey, inputDuration := range input {
		expectedDuration, ok := s.events[inputKey]
		if !ok || !s.approximatelySameDuration(inputDuration, expectedDuration) {
			return false
		}
	}

	return true
}

// if the duration is accurate to the 10ms precision, then this function returns true.
func (s *SequenceMonitor) approximatelySameDuration(d1, d2 time.Duration) bool {
	return (d1.Milliseconds() - d1.Milliseconds()%100) == (d2.Milliseconds() - d2.Milliseconds()%100)
}

func contextWithMonitor(monitor *SequenceMonitor) context.Context {
	//nolint:staticcheck
	return context.WithValue(context.Background(), "monitor", monitor)
}

// ShutdownMarker is a test shutdown handler that marks the completion of events using the SequenceMonitor from the context.
type ShutdownMarker struct {
	name  string
	err   error
	panic bool
}

func (s ShutdownMarker) Name() string {
	return s.name
}

func (s ShutdownMarker) HandleShutdown(ctx context.Context) error {
	monitor := ctx.Value("monitor").(*SequenceMonitor)
	if monitor == nil {
		panic("no monitor available")
	}

	time.Sleep(100 * time.Millisecond)
	monitor.Mark(s.name)
	if s.panic {
		panic("shutdown panic")
	}
	return s.err
}

func TestTriggerRuntime(t *testing.T) {
	testCases := []struct {
		Name              string
		Setup             func(*shutdown.Manager)
		Timeout           time.Duration
		ExpectedEventTime EventTime
	}{
		{
			// in this test, since all steps are parallel, they should all complete at the 10ms mark.
			Name: "AllParallel",
			Setup: func(m *shutdown.Manager) {
				m.AddSteps(ShutdownMarker{name: "a"}, ShutdownMarker{name: "b"})
				m.AddSteps(ShutdownMarker{name: "c"})
			},
			ExpectedEventTime: MustParseEventTime("a:100ms,b:100ms,c:100ms"),
		},
		{
			// in this test, the first two are in one parallel group, the second group is also in a parallel group so the
			// first group should complete around the same time and after that the second group should start and finish
			// around the same time.
			Name: "TwoParallelSequences",
			Setup: func(m *shutdown.Manager) {
				m.AddSteps(ShutdownMarker{name: "a"}, ShutdownMarker{name: "b"})
				m.AddParallelSequence(ShutdownMarker{name: "c"}, ShutdownMarker{name: "d"})
			},
			ExpectedEventTime: MustParseEventTime("a:100ms,b:100ms,c:200ms,d:200ms"),
		},
		{
			// in this test, the first group runs in parallel and when they're all completed the second group runs in
			// sequence and one after each other. So a and b complete at the same time, after that c runs and completes then
			// d runs and completes.
			Name: "ParallelAndSequence",
			Setup: func(m *shutdown.Manager) {
				m.AddSteps(
					ShutdownMarker{name: "a"},
					ShutdownMarker{name: "b"})
				m.AddSequence(
					ShutdownMarker{name: "c"},
					ShutdownMarker{name: "d"})
			},
			ExpectedEventTime: MustParseEventTime("a:100ms,b:100ms,c:200ms,d:300ms"),
		},
		{
			// in this test, we have an ordered set of steps but one of them errors out.
			Name: "SequenceWithError",
			Setup: func(m *shutdown.Manager) {
				m.AddSequence(
					ShutdownMarker{name: "a", err: errors.New("failed")},
					ShutdownMarker{name: "b"})
			},
			ExpectedEventTime: MustParseEventTime("a:100ms,b:200ms"),
		},
		{
			// in this test, we have an ordered set of steps but one of them panics.
			Name: "SequenceWithPanic",
			Setup: func(m *shutdown.Manager) {
				m.AddSequence(
					ShutdownMarker{name: "a", panic: true},
					ShutdownMarker{name: "b"})
			},
			ExpectedEventTime: MustParseEventTime("a:100ms,b:200ms"),
		},
		{
			// in this test, timeout is tested. Shutdown should still get called and the first step should complete but the
			// second step should no longer block the execution.
			Name: "SequenceWithTimeout",
			Setup: func(m *shutdown.Manager) {
				m.AddSequence(
					ShutdownMarker{name: "a", err: errors.New("failed")},
					ShutdownMarker{name: "b"})
				m.SetTimeout(120 * time.Millisecond) // time to complete a but not b
			},
			ExpectedEventTime: MustParseEventTime("a:100ms"),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.Name, func(t *testing.T) {
			useLogger := true
			shutdownCalled := false
			for i := 0; i < 2; i++ {
				monitor := &SequenceMonitor{}
				pipeline := shutdown.New()
				var logger shutdown.Logger
				logData := &bytes.Buffer{}

				if useLogger {
					logger = LogWrapper{Writer: logData}
					pipeline.SetLogger(logger)
					useLogger = false
				} else {
					pipeline.SetLogger(nil)
				}

				if !shutdownCalled {
					pipeline.SetCompletionFunc(func() {
						shutdownCalled = true
					})
				}
				pipeline.SetTimeout(tt.Timeout)

				tt.Setup(pipeline)
				monitor.StartRecording()
				pipeline.Trigger(contextWithMonitor(monitor))
				if !monitor.Matches(tt.ExpectedEventTime) {
					t.Logf("log data:\n%s", logData.String())
					t.Fatalf("expected event time:\n%s\ngot:\n%s", tt.ExpectedEventTime, monitor.EventTime())
					return
				}
				if !shutdownCalled {
					t.Fatal("shutdown function did not get called")
				}
			}
		})
	}
}

func TestDefault(t *testing.T) {
	var d *shutdown.Manager
	wg := sync.WaitGroup{}
	wg.Add(10)
	broken := false
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			mgr := shutdown.Default()
			if d != nil && mgr != d {
				broken = true
			}
			d = mgr
		}()
	}
	wg.Wait()
	if broken {
		t.Fatal("default pointer changed")
		return
	}

	monitor := &SequenceMonitor{}
	monitor.StartRecording()

	shutdownCalled := false
	shutdown.AddSteps(ShutdownMarker{name: "1"}, ShutdownMarker{name: "2"})
	shutdown.AddSequence(ShutdownMarker{name: "3"}, ShutdownMarker{name: "4"})
	shutdown.AddParallelSequence(ShutdownMarker{name: "5"}, ShutdownMarker{name: "6"})
	shutdown.SetTimeout(350 * time.Millisecond)
	shutdown.SetLogger(nil)
	shutdown.SetCompletionFunc(func() {
		shutdownCalled = true
	})
	shutdown.Trigger(contextWithMonitor(monitor))
	if !shutdownCalled {
		t.Fatalf("shutdown did not get called")
	}

	expectation := MustParseEventTime("1:100ms,2:100ms,3:200ms,4:300ms")
	if !monitor.Matches(expectation) {
		t.Fatalf("expected:\n%s\ngot:\n%s", expectation, monitor.EventTime())
	}
}

func TestHandlerFunc(t *testing.T) {
	// just make sure the handle functions get compiled.
	manager := shutdown.New()
	manager.AddSteps(shutdown.HandlerFuncWithName("no-context-no-error", func() {}))
	manager.AddSteps(shutdown.HandlerFuncWithName("context-no-error", func(context.Context) {}))
	manager.AddSteps(shutdown.HandlerFuncWithName("no-context-error", func(context.Context) error { return nil }))
	manager.AddSteps(shutdown.HandlerFuncWithName("context-error", func(context.Context) error { return nil }))
}
