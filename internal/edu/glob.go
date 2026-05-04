package edu

import (
	"path"
)

// MatchAllowedFilesGlob returns files that do NOT match the glob. Uses path.Match
// (POSIX-shell-style, "/" separator, no recursive "**"). Empty or invalid glob
// blocks everything (returns all input as violations).
func MatchAllowedFilesGlob(glob string, files []string) []string {
	if len(files) == 0 {
		return nil
	}
	var violations []string
	for _, f := range files {
		if glob == "" {
			violations = append(violations, f)
			continue
		}
		ok, err := path.Match(glob, f)
		if err != nil || !ok {
			violations = append(violations, f)
		}
	}
	return violations
}
