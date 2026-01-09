package tui

import "time"

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
