package edu

import (
	"context"
	"fmt"

	actions_model "forgejo.org/models/actions"
	"forgejo.org/modules/actions"
)

// ReadRunLogLines returns the flat list of log content lines across all jobs
// (and their latest tasks) of the given action run. Best-effort: jobs without
// a TaskID and per-task ReadLogs errors are skipped silently; a top-level
// GetRunJobsByRunID error is returned.
func (a *ForgejoAdapter) ReadRunLogLines(ctx context.Context, runID int64) ([]string, error) {
	jobs, err := actions_model.GetRunJobsByRunID(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("get run jobs %d: %w", runID, err)
	}
	var lines []string
	for _, job := range jobs {
		if job.TaskID == 0 {
			continue
		}
		task, err := actions_model.GetTaskByID(ctx, job.TaskID)
		if err != nil {
			continue
		}
		rows, err := actions.ReadLogs(ctx, task.LogInStorage, task.LogFilename, 0, -1)
		if err != nil {
			continue
		}
		for _, row := range rows {
			lines = append(lines, row.Content)
		}
	}
	return lines, nil
}
