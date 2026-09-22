package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// JobsTotal counts the number of jobs created and completed by type and status.
	JobsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "workflow_engine",
			Subsystem: "jobs",
			Name:      "total",
			Help:      "Total number of jobs by type and terminal status.",
		},
		[]string{"type", "status"},
	)

	// JobDuration measures the execution time of jobs from claim to completion.
	JobDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "workflow_engine",
			Subsystem: "jobs",
			Name:      "duration_seconds",
			Help:      "Duration of job executions in seconds.",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		},
		[]string{"type", "status"},
	)

	// QueueDepth records the backlog depth in Redis by queue and priority.
	QueueDepth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "workflow_engine",
			Subsystem: "queue",
			Name:      "depth",
			Help:      "Number of pending jobs in Redis queue by priority.",
		},
		[]string{"queue", "priority"},
	)

	// ActiveWorkers records the number of alive worker processes with active heartbeats.
	ActiveWorkers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "workflow_engine",
			Subsystem: "workers",
			Name:      "active",
			Help:      "Number of active worker nodes reporting heartbeats.",
		},
	)

	// WorkflowsTotal counts workflows by status.
	WorkflowsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "workflow_engine",
			Subsystem: "workflows",
			Name:      "total",
			Help:      "Total number of workflows created and completed by status.",
		},
		[]string{"status"},
	)
)

// RecordJobCreated increments the counter when a job is enqueued.
func RecordJobCreated(jobType string) {
	JobsTotal.WithLabelValues(jobType, "QUEUED").Inc()
}

// RecordJobFinished records the execution duration and terminal status of a job.
func RecordJobFinished(jobType, status string, duration time.Duration) {
	JobsTotal.WithLabelValues(jobType, status).Inc()
	JobDuration.WithLabelValues(jobType, status).Observe(duration.Seconds())
}

// UpdateQueueDepth sets the current depth for a specific priority queue.
func UpdateQueueDepth(queueName string, priority int, count int64) {
	QueueDepth.WithLabelValues(queueName, strconv.Itoa(priority)).Set(float64(count))
}

// SetActiveWorkers sets the current active worker count.
func SetActiveWorkers(count int) {
	ActiveWorkers.Set(float64(count))
}

// RecordWorkflow increments the workflow counter for a given status.
func RecordWorkflow(status string) {
	WorkflowsTotal.WithLabelValues(status).Inc()
}
