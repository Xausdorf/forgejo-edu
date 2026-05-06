package edu

import "testing"

func TestFormatSubmissionPRTitle(t *testing.T) {
	tests := []struct {
		name      string
		taskName  string
		groupName string
		want      string
	}{
		{"with group", "multiplication", "SE-241", "[SE-241] Submit: multiplication"},
		{"empty group", "multiplication", "", "Submit: multiplication"},
		{"group with spaces", "lab1", "Group A", "[Group A] Submit: lab1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSubmissionPRTitle(tt.taskName, tt.groupName)
			if got != tt.want {
				t.Errorf("formatSubmissionPRTitle(%q, %q) = %q, want %q", tt.taskName, tt.groupName, got, tt.want)
			}
		})
	}
}
