package metrics

import (
	"testing"
	"time"
)

func TestMetrics_Recording(t *testing.T) {
	RecordJobCreated("send_email")
	RecordJobFinished("send_email", "SUCCEEDED", 15*time.Millisecond)
	UpdateQueueDepth("default", 1, 5)
	SetActiveWorkers(3)
	RecordWorkflow("RUNNING")
	RecordWorkflow("SUCCEEDED")
}
