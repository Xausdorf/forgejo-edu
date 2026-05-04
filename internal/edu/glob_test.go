package edu

import (
	"reflect"
	"testing"
)

func TestMatchAllowedFilesGlob(t *testing.T) {
	cases := []struct {
		name           string
		glob           string
		files          []string
		wantViolations []string
	}{
		{
			name:           "exact_match_ok",
			glob:           "tasks/multiplication/multiplication.cpp",
			files:          []string{"tasks/multiplication/multiplication.cpp"},
			wantViolations: nil,
		},
		{
			name:           "wildcard_match_ok",
			glob:           "tasks/multiplication/*.cpp",
			files:          []string{"tasks/multiplication/a.cpp", "tasks/multiplication/b.cpp"},
			wantViolations: nil,
		},
		{
			name:           "single_violation",
			glob:           "tasks/multiplication/*.cpp",
			files:          []string{"tasks/multiplication/a.cpp", "tasks/other/b.cpp"},
			wantViolations: []string{"tasks/other/b.cpp"},
		},
		{
			name:           "all_violations",
			glob:           "tasks/multiplication/*.cpp",
			files:          []string{"README.md", ".gitignore"},
			wantViolations: []string{"README.md", ".gitignore"},
		},
		{
			name:           "empty_files_no_violations",
			glob:           "tasks/multiplication/*.cpp",
			files:          nil,
			wantViolations: nil,
		},
		{
			name:           "empty_glob_blocks_everything",
			glob:           "",
			files:          []string{"foo"},
			wantViolations: []string{"foo"},
		},
		{
			name:           "subdirectory_not_matched_by_single_star",
			glob:           "tasks/multiplication/*",
			files:          []string{"tasks/multiplication/sub/file.cpp"},
			wantViolations: []string{"tasks/multiplication/sub/file.cpp"},
		},
		{
			name:           "invalid_glob_treated_as_violation",
			glob:           "tasks/[",
			files:          []string{"tasks/x"},
			wantViolations: []string{"tasks/x"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MatchAllowedFilesGlob(tc.glob, tc.files)
			if !reflect.DeepEqual(got, tc.wantViolations) {
				t.Fatalf("MatchAllowedFilesGlob(%q, %v) = %v, want %v",
					tc.glob, tc.files, got, tc.wantViolations)
			}
		})
	}
}
