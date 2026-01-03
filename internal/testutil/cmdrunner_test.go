package testutil

import (
	"context"
	"errors"
	"testing"
)

func TestMockRunner_BasicResponse(t *testing.T) {
	mock := NewMockRunner()
	mock.On([]string{"gt", "status"}, []byte(`{"name":"test"}`), nil, nil)

	stdout, stderr, err := mock.Exec(context.Background(), "/tmp", "gt", "status", "--json")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if string(stderr) != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	if string(stdout) != `{"name":"test"}` {
		t.Errorf("unexpected stdout: %s", stdout)
	}
}

func TestMockRunner_ErrorResponse(t *testing.T) {
	mock := NewMockRunner()
	expectedErr := errors.New("command failed")
	mock.On([]string{"gt", "status"}, nil, []byte("error details"), expectedErr)

	_, stderr, err := mock.Exec(context.Background(), "/tmp", "gt", "status")

	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
	if string(stderr) != "error details" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
}

func TestMockRunner_WildcardMatch(t *testing.T) {
	mock := NewMockRunner()
	mock.On([]string{"gt", "mq", "list", "*"}, []byte(`[]`), nil, nil)

	// Should match any rig name
	stdout, _, err := mock.Exec(context.Background(), "/tmp", "gt", "mq", "list", "perch", "--json")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if string(stdout) != "[]" {
		t.Errorf("unexpected stdout: %s", stdout)
	}

	// Should also match different rig name
	stdout, _, err = mock.Exec(context.Background(), "/tmp", "gt", "mq", "list", "sidekick")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if string(stdout) != "[]" {
		t.Errorf("unexpected stdout: %s", stdout)
	}
}

func TestMockRunner_OnFunc(t *testing.T) {
	mock := NewMockRunner()
	callCount := 0

	mock.OnFunc([]string{"gt", "polecat"}, func(args []string) ([]byte, []byte, error) {
		callCount++
		return []byte("called"), nil, nil
	})

	mock.Exec(context.Background(), "/tmp", "gt", "polecat", "list")
	mock.Exec(context.Background(), "/tmp", "gt", "polecat", "status")

	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

func TestMockRunner_OnMatcher(t *testing.T) {
	mock := NewMockRunner()

	// Custom matcher: match any bd command
	mock.OnMatcher(func(args []string) bool {
		return len(args) > 0 && args[0] == "bd"
	}, func(args []string) ([]byte, []byte, error) {
		return []byte("bd response"), nil, nil
	})

	stdout, _, err := mock.Exec(context.Background(), "/tmp", "bd", "list", "--json")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if string(stdout) != "bd response" {
		t.Errorf("unexpected stdout: %s", stdout)
	}
}

func TestMockRunner_DefaultResponse(t *testing.T) {
	mock := NewMockRunner()
	mock.DefaultStdout = []byte("default")
	mock.DefaultStderr = []byte("default error")
	mock.DefaultError = errors.New("default error")

	// No handlers registered, should return defaults
	stdout, stderr, err := mock.Exec(context.Background(), "/tmp", "unknown", "command")

	if string(stdout) != "default" {
		t.Errorf("expected default stdout, got %s", stdout)
	}
	if string(stderr) != "default error" {
		t.Errorf("expected default stderr, got %s", stderr)
	}
	if err == nil || err.Error() != "default error" {
		t.Errorf("expected default error, got %v", err)
	}
}

func TestMockRunner_LastHandlerWins(t *testing.T) {
	mock := NewMockRunner()
	mock.On([]string{"gt", "status"}, []byte("first"), nil, nil)
	mock.On([]string{"gt", "status"}, []byte("second"), nil, nil)

	stdout, _, _ := mock.Exec(context.Background(), "/tmp", "gt", "status")

	if string(stdout) != "second" {
		t.Errorf("expected 'second' (last handler), got %s", stdout)
	}
}

func TestMockRunner_CallTracking(t *testing.T) {
	mock := NewMockRunner()
	mock.DefaultStdout = []byte("")

	// Make some calls
	mock.Exec(context.Background(), "/tmp", "gt", "status")
	mock.Exec(context.Background(), "/home", "bd", "list")
	mock.Exec(context.Background(), "/tmp", "gt", "polecat", "list")

	if !mock.Called() {
		t.Error("expected Called() to be true")
	}

	if !mock.CalledWith([]string{"gt", "status"}) {
		t.Error("expected CalledWith gt status to be true")
	}

	if !mock.CalledWith([]string{"bd"}) {
		t.Error("expected CalledWith bd to be true")
	}

	if mock.CalledWith([]string{"unknown"}) {
		t.Error("expected CalledWith unknown to be false")
	}

	calls := mock.Calls()
	if len(calls) != 3 {
		t.Errorf("expected 3 calls, got %d", len(calls))
	}

	// Verify call details
	if calls[0].WorkDir != "/tmp" || calls[0].Args[0] != "gt" {
		t.Errorf("unexpected first call: %v", calls[0])
	}
	if calls[1].WorkDir != "/home" || calls[1].Args[0] != "bd" {
		t.Errorf("unexpected second call: %v", calls[1])
	}
}

func TestMockRunner_CallCount(t *testing.T) {
	mock := NewMockRunner()
	mock.DefaultStdout = []byte("")

	mock.Exec(context.Background(), "/tmp", "gt", "status")
	mock.Exec(context.Background(), "/tmp", "gt", "status")
	mock.Exec(context.Background(), "/tmp", "bd", "list")

	if count := mock.CallCount([]string{"gt", "status"}); count != 2 {
		t.Errorf("expected 2 calls to gt status, got %d", count)
	}

	if count := mock.CallCount([]string{"bd"}); count != 1 {
		t.Errorf("expected 1 call to bd, got %d", count)
	}

	if count := mock.CallCount([]string{"gt"}); count != 2 {
		t.Errorf("expected 2 calls matching gt, got %d", count)
	}
}

func TestMockRunner_Reset(t *testing.T) {
	mock := NewMockRunner()
	mock.On([]string{"gt"}, []byte("response"), nil, nil)

	mock.Exec(context.Background(), "/tmp", "gt", "status")

	if !mock.Called() {
		t.Error("expected Called() to be true before reset")
	}

	mock.Reset()

	if mock.Called() {
		t.Error("expected Called() to be false after reset")
	}

	// Handlers should still work
	stdout, _, _ := mock.Exec(context.Background(), "/tmp", "gt", "status")
	if string(stdout) != "response" {
		t.Error("handler should still work after reset")
	}
}

func TestMockCall_String(t *testing.T) {
	call := MockCall{
		WorkDir: "/tmp/town",
		Args:    []string{"gt", "status", "--json"},
	}

	expected := "/tmp/town: gt status --json"
	if call.String() != expected {
		t.Errorf("expected %q, got %q", expected, call.String())
	}
}

func TestMatchArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		pattern []string
		want    bool
	}{
		{"exact match", []string{"gt", "status"}, []string{"gt", "status"}, true},
		{"prefix match", []string{"gt", "status", "--json"}, []string{"gt", "status"}, true},
		{"wildcard", []string{"gt", "mq", "list", "perch"}, []string{"gt", "mq", "list", "*"}, true},
		{"no match", []string{"gt", "status"}, []string{"bd", "list"}, false},
		{"too short", []string{"gt"}, []string{"gt", "status"}, false},
		{"empty pattern", []string{"gt", "status"}, []string{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchArgs(tt.args, tt.pattern)
			if got != tt.want {
				t.Errorf("matchArgs(%v, %v) = %v, want %v", tt.args, tt.pattern, got, tt.want)
			}
		})
	}
}
