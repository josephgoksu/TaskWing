package task

import (
	"strings"
	"testing"
)

func TestTask_Validate(t *testing.T) {
	tests := []struct {
		name    string
		task    Task
		wantErr bool
	}{
		{
			name: "valid task",
			task: Task{
				Title:       "Valid Task",
				Description: "Valid Description",
				Priority:    50,
			},
			wantErr: false,
		},
		{
			name: "empty title",
			task: Task{
				Title:       "",
				Description: "Valid Description",
				Priority:    50,
			},
			wantErr: true, // title required
		},
		{
			name: "long title",
			task: Task{
				Title:       strings.Repeat("a", 201),
				Description: "Valid Description",
				Priority:    50,
			},
			wantErr: true, // max 200
		},
		{
			name: "empty description",
			task: Task{
				Title:       "Valid Task",
				Description: "",
				Priority:    50,
			},
			wantErr: true, // description required
		},
		{
			name: "priority too low",
			task: Task{
				Title:       "Valid Task",
				Description: "Valid Description",
				Priority:    -1,
			},
			wantErr: true, // 0-100
		},
		{
			name: "priority too high",
			task: Task{
				Title:       "Valid Task",
				Description: "Valid Description",
				Priority:    101,
			},
			wantErr: true, // 0-100
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.task.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Task.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
