package domain

import (
	"testing"
)

func TestValidateDAG(t *testing.T) {
	tests := []struct {
		name    string
		tasks   []CreateWorkflowTaskRequest
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty tasks",
			tasks:   []CreateWorkflowTaskRequest{},
			wantErr: true,
			errMsg:  "workflow must contain at least one task",
		},
		{
			name: "empty task name",
			tasks: []CreateWorkflowTaskRequest{
				{Name: "", Type: "email"},
			},
			wantErr: true,
			errMsg:  "task name is required",
		},
		{
			name: "empty task type",
			tasks: []CreateWorkflowTaskRequest{
				{Name: "task1", Type: ""},
			},
			wantErr: true,
			errMsg:  `task "task1" must specify a type`,
		},
		{
			name: "duplicate task name",
			tasks: []CreateWorkflowTaskRequest{
				{Name: "step1", Type: "email"},
				{Name: "step1", Type: "report"},
			},
			wantErr: true,
			errMsg:  `duplicate task name: "step1"`,
		},
		{
			name: "self dependency",
			tasks: []CreateWorkflowTaskRequest{
				{Name: "step1", Type: "email", Dependencies: []string{"step1"}},
			},
			wantErr: true,
			errMsg:  `task "step1" cannot depend on itself`,
		},
		{
			name: "unknown dependency",
			tasks: []CreateWorkflowTaskRequest{
				{Name: "step1", Type: "email", Dependencies: []string{"ghost_step"}},
			},
			wantErr: true,
			errMsg:  `task "step1" depends on unknown task "ghost_step"`,
		},
		{
			name: "simple 2-node cycle",
			tasks: []CreateWorkflowTaskRequest{
				{Name: "A", Type: "t", Dependencies: []string{"B"}},
				{Name: "B", Type: "t", Dependencies: []string{"A"}},
			},
			wantErr: true,
			errMsg:  "cycle detected in workflow dependencies",
		},
		{
			name: "3-node cycle",
			tasks: []CreateWorkflowTaskRequest{
				{Name: "A", Type: "t", Dependencies: []string{"C"}},
				{Name: "B", Type: "t", Dependencies: []string{"A"}},
				{Name: "C", Type: "t", Dependencies: []string{"B"}},
			},
			wantErr: true,
			errMsg:  "cycle detected in workflow dependencies",
		},
		{
			name: "valid linear workflow",
			tasks: []CreateWorkflowTaskRequest{
				{Name: "create_user", Type: "db"},
				{Name: "send_welcome", Type: "email", Dependencies: []string{"create_user"}},
				{Name: "provision_s3", Type: "cloud", Dependencies: []string{"create_user"}},
				{Name: "complete", Type: "notify", Dependencies: []string{"send_welcome", "provision_s3"}},
			},
			wantErr: false,
		},
		{
			name: "valid diamond DAG",
			tasks: []CreateWorkflowTaskRequest{
				{Name: "A", Type: "t"},
				{Name: "B", Type: "t", Dependencies: []string{"A"}},
				{Name: "C", Type: "t", Dependencies: []string{"A"}},
				{Name: "D", Type: "t", Dependencies: []string{"B", "C"}},
			},
			wantErr: false,
		},
		{
			name: "valid disconnected parallel branches",
			tasks: []CreateWorkflowTaskRequest{
				{Name: "branch1_step1", Type: "t"},
				{Name: "branch1_step2", Type: "t", Dependencies: []string{"branch1_step1"}},
				{Name: "branch2_step1", Type: "t"},
				{Name: "branch2_step2", Type: "t", Dependencies: []string{"branch2_step1"}},
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateDAG(tc.tasks)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateDAG() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr && err.Error() != tc.errMsg {
				t.Errorf("expected error %q, got %q", tc.errMsg, err.Error())
			}
		})
	}
}
