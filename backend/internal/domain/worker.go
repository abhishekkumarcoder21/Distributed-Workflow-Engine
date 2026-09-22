package domain

import "time"

type WorkerStatus string

const (
	WorkerStatusActive   WorkerStatus = "ACTIVE"
	WorkerStatusDraining WorkerStatus = "DRAINING"
	WorkerStatusOffline  WorkerStatus = "OFFLINE"
)

type Worker struct {
	ID             string       `json:"id" db:"id"`
	Status         WorkerStatus `json:"status" db:"status"`
	LastHeartbeat  time.Time    `json:"last_heartbeat" db:"last_heartbeat"`
	StartedAt      time.Time    `json:"started_at" db:"started_at"`
	CompletedCount int          `json:"completed_count" db:"completed_count"`
	FailedCount    int          `json:"failed_count" db:"failed_count"`
	CreatedAt      time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at" db:"updated_at"`
}
