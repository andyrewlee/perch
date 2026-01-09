package tui

import (
	"strings"
	"time"
)

// nowFunc is overridden in tests for deterministic output.
var nowFunc = time.Now

func now() time.Time {
	return nowFunc()
}

func since(t time.Time) time.Duration {
	return now().Sub(t)
}

// setNow is used in tests to pin time.
func setNow(t time.Time) func() {
	prev := nowFunc
	nowFunc = func() time.Time { return t }
	return func() { nowFunc = prev }
}

// parseRigFromAgentAddress extracts the rig name from an agent address.
// Addresses are typically in the format: "rig/role/name" (e.g., "perch/polecat/ace")
// or "rig/name" for some agents. The first component is always the rig name.
func parseRigFromAgentAddress(address string) string {
	parts := strings.Split(address, "/")
	if len(parts) >= 1 {
		return parts[0]
	}
	return "unknown"
}
