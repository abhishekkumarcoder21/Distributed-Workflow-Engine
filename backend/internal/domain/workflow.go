package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type WorkflowStatus string

const (
	WorkflowStatusPending   WorkflowStatus = "PENDING"
	WorkflowStatusRunning   WorkflowStatus = "RUNNING"
	WorkflowStatusSucceeded WorkflowStatus = "SUCCEEDED"
	WorkflowStatusFailed    WorkflowStatus = "FAILED"
	WorkflowStatusCancelled WorkflowStatus = "CANCELLED"
)

func (s WorkflowStatus) IsTerminal() bool {
	return s == WorkflowStatusSucceeded || s == WorkflowStatusFailed || s == WorkflowStatusCancelled
}

type WorkflowTaskStatus string

const (
	WorkflowTaskStatusPending   WorkflowTaskStatus = "PENDING"
	WorkflowTaskStatusRunning   WorkflowTaskStatus = "RUNNING"
	WorkflowTaskStatusSucceeded WorkflowTaskStatus = "SUCCEEDED"
	WorkflowTaskStatusFailed    WorkflowTaskStatus = "FAILED"
	WorkflowTaskStatusCancelled WorkflowTaskStatus = "CANCELLED"
)

func (s WorkflowTaskStatus) IsTerminal() bool {
	return s == WorkflowTaskStatusSucceeded || s == WorkflowTaskStatusFailed || s == WorkflowTaskStatusCancelled
}

type Workflow struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`
	Status      WorkflowStatus `json:"status"`
	Error       *string        `json:"error,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type WorkflowTask struct {
	ID           uuid.UUID          `json:"id"`
	WorkflowID   uuid.UUID          `json:"workflow_id"`
	JobID        *uuid.UUID         `json:"job_id,omitempty"`
	Name         string             `json:"name"`
	Type         string             `json:"type"`
	Payload      json.RawMessage    `json:"payload"`
	Status       WorkflowTaskStatus `json:"status"`
	Dependencies []string           `json:"dependencies"`
	Error        *string            `json:"error,omitempty"`
	CreatedAt    time.Time          `json:"created_at"`
	StartedAt    *time.Time         `json:"started_at,omitempty"`
	CompletedAt  *time.Time         `json:"completed_at,omitempty"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

type WorkflowWithTasks struct {
	Workflow
	Tasks []WorkflowTask `json:"tasks"`
}

type CreateWorkflowTaskRequest struct {
	Name         string          `json:"name"`
	Type         string          `json:"type"`
	Payload      json.RawMessage `json:"payload"`
	Priority     *int            `json:"priority,omitempty"`
	Dependencies []string        `json:"dependencies,omitempty"`
}

type CreateWorkflowRequest struct {
	Name  string                      `json:"name"`
	Tasks []CreateWorkflowTaskRequest `json:"tasks"`
}

type WorkflowFilter struct {
	Status *WorkflowStatus
	Limit  int
	Offset int
}

// ValidateDAG validates that the tasks form a valid Directed Acyclic Graph.
// Checks for:
// - At least one task
// - Unique non-empty task names
// - Non-empty task types
// - Valid dependencies (exist in task set, no self-references)
// - No cycles (topological sort via Kahn's algorithm)
func ValidateDAG(tasks []CreateWorkflowTaskRequest) error {
	if len(tasks) == 0 {
		return errors.New("workflow must contain at least one task")
	}

	taskNames := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		if t.Name == "" {
			return errors.New("task name is required")
		}
		if t.Type == "" {
			return fmt.Errorf("task %q must specify a type", t.Name)
		}
		if taskNames[t.Name] {
			return fmt.Errorf("duplicate task name: %q", t.Name)
		}
		taskNames[t.Name] = true
	}

	inDegree := make(map[string]int, len(tasks))
	adjacency := make(map[string][]string, len(tasks))

	for _, t := range tasks {
		inDegree[t.Name] = 0
	}

	for _, t := range tasks {
		for _, dep := range t.Dependencies {
			if dep == t.Name {
				return fmt.Errorf("task %q cannot depend on itself", t.Name)
			}
			if !taskNames[dep] {
				return fmt.Errorf("task %q depends on unknown task %q", t.Name, dep)
			}
			adjacency[dep] = append(adjacency[dep], t.Name)
			inDegree[t.Name]++
		}
	}

	// Kahn's algorithm for cycle detection
	queue := make([]string, 0, len(tasks))
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	visitedCount := 0
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		visitedCount++

		for _, neighbor := range adjacency[curr] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if visitedCount != len(tasks) {
		return errors.New("cycle detected in workflow dependencies")
	}

	return nil
}
