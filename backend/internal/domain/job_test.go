package domain

import "testing"

func TestJobStatus_IsTerminal(t *testing.T) {
	terminal := []JobStatus{
		JobStatusSucceeded,
		JobStatusFailed,
		JobStatusCancelled,
		JobStatusTimeout,
		JobStatusDeadLetter,
	}
	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Errorf("expected %s to be terminal", s)
		}
	}

	nonTerminal := []JobStatus{
		JobStatusQueued,
		JobStatusRunning,
		JobStatusRetrying,
	}
	for _, s := range nonTerminal {
		if s.IsTerminal() {
			t.Errorf("expected %s to be non-terminal", s)
		}
	}
}
