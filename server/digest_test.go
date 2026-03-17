package main

import (
	"testing"
)

// === Skipped test stubs for Plan 03-02 ===

func TestDigestExecution_Daily(t *testing.T) {
	t.Skip("Plan 03-02: daily digest sends summary at configured hour")
}

func TestDigestExecution_Weekly(t *testing.T) {
	t.Skip("Plan 03-02: weekly digest sends summary on configured weekday and hour")
}

func TestDigestExecution_NotDueYet(t *testing.T) {
	t.Skip("Plan 03-02: digest check skips channels not yet due")
}

func TestDigestContent(t *testing.T) {
	t.Skip("Plan 03-02: digest content includes state counts and recent changes")
}
