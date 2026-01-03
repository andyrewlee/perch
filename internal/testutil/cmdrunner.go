// Package testutil provides testing utilities for Gas Town TUI.
package testutil

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// CommandRunner matches data.CommandRunner interface.
// Defined here to avoid import cycles.
type CommandRunner interface {
	Exec(ctx context.Context, workDir string, args ...string) (stdout, stderr []byte, err error)
}

// Ensure MockRunner implements CommandRunner.
var _ CommandRunner = (*MockRunner)(nil)

// MockRunner simulates command execution for testing.
// It returns pre-configured responses based on command patterns.
type MockRunner struct {
	mu       sync.Mutex
	handlers []MockHandler
	calls    []MockCall

	// Default response when no handler matches
	DefaultStdout []byte
	DefaultStderr []byte
	DefaultError  error
}

// MockHandler defines a response for a command pattern.
type MockHandler struct {
	// Match returns true if this handler should handle the command.
	// args contains the full command including the program name.
	Match func(args []string) bool

	// Response returns the mock response.
	// Called only if Match returns true.
	Response func(ctx context.Context, args []string) (stdout, stderr []byte, err error)
}

// MockCall records a command invocation for verification.
type MockCall struct {
	WorkDir string
	Args    []string
}

// NewMockRunner creates a new mock runner.
func NewMockRunner() *MockRunner {
	return &MockRunner{}
}

// On registers a handler for commands matching the given pattern.
// Pattern matching is done via prefix matching on args.
func (m *MockRunner) On(pattern []string, stdout []byte, stderr []byte, err error) *MockRunner {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.handlers = append(m.handlers, MockHandler{
		Match: func(args []string) bool {
			return matchArgs(args, pattern)
		},
		Response: func(ctx context.Context, args []string) ([]byte, []byte, error) {
			return stdout, stderr, err
		},
	})
	return m
}

// OnFunc registers a handler with a custom response function.
func (m *MockRunner) OnFunc(pattern []string, fn func(args []string) ([]byte, []byte, error)) *MockRunner {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.handlers = append(m.handlers, MockHandler{
		Match: func(args []string) bool {
			return matchArgs(args, pattern)
		},
		Response: func(ctx context.Context, args []string) ([]byte, []byte, error) {
			return fn(args)
		},
	})
	return m
}

// OnMatcher registers a handler with a custom matcher function.
func (m *MockRunner) OnMatcher(match func(args []string) bool, fn func(args []string) ([]byte, []byte, error)) *MockRunner {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.handlers = append(m.handlers, MockHandler{
		Match: match,
		Response: func(ctx context.Context, args []string) ([]byte, []byte, error) {
			return fn(args)
		},
	})
	return m
}

// Exec implements CommandRunner.
func (m *MockRunner) Exec(ctx context.Context, workDir string, args ...string) ([]byte, []byte, error) {
	m.mu.Lock()
	m.calls = append(m.calls, MockCall{WorkDir: workDir, Args: args})

	// Find matching handler (last registered wins)
	var handler *MockHandler
	for i := len(m.handlers) - 1; i >= 0; i-- {
		if m.handlers[i].Match(args) {
			handler = &m.handlers[i]
			break
		}
	}
	m.mu.Unlock()

	if handler != nil {
		return handler.Response(ctx, args)
	}

	// Return default response
	return m.DefaultStdout, m.DefaultStderr, m.DefaultError
}

// Calls returns all recorded command invocations.
func (m *MockRunner) Calls() []MockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]MockCall, len(m.calls))
	copy(result, m.calls)
	return result
}

// Reset clears all recorded calls (but keeps handlers).
func (m *MockRunner) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = nil
}

// Called returns true if any command was invoked.
func (m *MockRunner) Called() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls) > 0
}

// CalledWith returns true if a command matching the pattern was invoked.
func (m *MockRunner) CalledWith(pattern []string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, call := range m.calls {
		if matchArgs(call.Args, pattern) {
			return true
		}
	}
	return false
}

// CallCount returns the number of calls matching the pattern.
func (m *MockRunner) CallCount(pattern []string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, call := range m.calls {
		if matchArgs(call.Args, pattern) {
			count++
		}
	}
	return count
}

// matchArgs returns true if args matches the pattern.
// Pattern elements must match args prefix, with "*" as wildcard.
func matchArgs(args, pattern []string) bool {
	if len(args) < len(pattern) {
		return false
	}
	for i, p := range pattern {
		if p != "*" && p != args[i] {
			return false
		}
	}
	return true
}

// String returns a debug representation of the call.
func (c MockCall) String() string {
	return fmt.Sprintf("%s: %s", c.WorkDir, strings.Join(c.Args, " "))
}
