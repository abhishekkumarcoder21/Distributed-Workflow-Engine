package domain

import (
	"fmt"
	"testing"
)

// BenchmarkDAG_Validation measures the performance of Kahn's algorithm
// on DAGs of varying sizes: 10 nodes, 50 nodes, and 100 nodes.
func BenchmarkDAG_Validation_10Nodes(b *testing.B) {
	tasks := generateDAGTasks(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ValidateDAG(tasks); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDAG_Validation_50Nodes(b *testing.B) {
	tasks := generateDAGTasks(50)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ValidateDAG(tasks); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDAG_Validation_100Nodes(b *testing.B) {
	tasks := generateDAGTasks(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ValidateDAG(tasks); err != nil {
			b.Fatal(err)
		}
	}
}

func generateDAGTasks(n int) []CreateWorkflowTaskRequest {
	tasks := make([]CreateWorkflowTaskRequest, n)
	for i := 0; i < n; i++ {
		var deps []string
		if i > 0 {
			// Each task depends on the previous task (or 2 previous tasks)
			deps = append(deps, fmt.Sprintf("task_%d", i-1))
			if i > 1 {
				deps = append(deps, fmt.Sprintf("task_%d", i-2))
			}
		}
		tasks[i] = CreateWorkflowTaskRequest{
			Name:         fmt.Sprintf("task_%d", i),
			Type:         "compute",
			Dependencies: deps,
		}
	}
	return tasks
}
