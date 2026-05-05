package edu

import (
	"context"
	"fmt"
	"strings"

	user_model "forgejo.org/models/user"
)

// systemMergeDoerName is the Forgejo account whose identity is used for the
// internal git merge call when a TA presses Merge in the edu UI. Same account
// the notifier uses for system PR creation/comments (see notifier.go:17).
const systemMergeDoerName = "eduadmin"

func (s *service) GetTestResults(ctx context.Context, submissionID int64) ([]*TestResult, error) {
	return s.repo.GetTestResultsBySubmission(ctx, submissionID)
}

func (s *service) GetLatestTestResult(ctx context.Context, submissionID int64) (*TestResult, error) {
	return s.repo.GetLatestTestResult(ctx, submissionID)
}

func (s *service) GetSubmissionByID(ctx context.Context, id int64) (*Submission, error) {
	return s.repo.GetSubmissionByID(ctx, id)
}

// ApproveSubmission validates the grade and records the TA's approval:
// status=approved, manual_grade=true, grade/comment/grader stored.
func (s *service) ApproveSubmission(ctx context.Context, submissionID int64, grade int, comment string, gradedByID int64) error {
	if grade < 0 || grade > 100 {
		return fmt.Errorf("grade must be between 0 and 100")
	}
	if len(comment) > 10000 {
		return fmt.Errorf("comment is too long (max 10000 chars)")
	}
	return s.repo.ApproveSubmission(ctx, submissionID, grade, comment, gradedByID)
}

// MergeSubmission performs the git merge of the submission PR via eduadmin
// and flips the status to merged. Refuses if the submission is not approved
// or has no PR.
func (s *service) MergeSubmission(ctx context.Context, submissionID int64) error {
	if s.pulls == nil {
		return fmt.Errorf("pull request service not configured")
	}
	sub, err := s.repo.GetSubmissionByID(ctx, submissionID)
	if err != nil {
		return fmt.Errorf("load submission: %w", err)
	}
	if sub == nil {
		return fmt.Errorf("submission %d not found", submissionID)
	}
	if sub.Status != StatusSubmissionApproved {
		return fmt.Errorf("submission must be approved before merging (current status: %s)", sub.Status)
	}
	if sub.PullRequestID == 0 {
		return fmt.Errorf("submission has no pull request")
	}

	doer, err := s.users.GetUserByName(ctx, systemMergeDoerName)
	if err != nil || doer == nil {
		return fmt.Errorf("system doer %q not found: %w", systemMergeDoerName, err)
	}

	if err := s.pulls.MergePullRequest(ctx, MergePullRequestOptions{
		PullRequestID: sub.PullRequestID,
		Doer:          doer,
		MergeStyle:    "merge",
		Message:       fmt.Sprintf("Merged via Edu UI by approval (submission %d).", sub.ID),
	}); err != nil {
		return fmt.Errorf("merge PR %d: %w", sub.PullRequestID, err)
	}

	if err := s.repo.MarkSubmissionMerged(ctx, sub.ID); err != nil {
		return fmt.Errorf("mark merged: %w", err)
	}
	return nil
}

// AddSubmissionComment posts a plain comment in the submission's PR thread on
// behalf of the calling user (TA / teacher).
func (s *service) AddSubmissionComment(ctx context.Context, submissionID int64, body string, doer *user_model.User) error {
	if s.pulls == nil {
		return fmt.Errorf("pull request service not configured")
	}
	if doer == nil {
		return fmt.Errorf("doer is required")
	}
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("comment cannot be empty")
	}
	sub, err := s.repo.GetSubmissionByID(ctx, submissionID)
	if err != nil {
		return fmt.Errorf("load submission: %w", err)
	}
	if sub == nil {
		return fmt.Errorf("submission %d not found", submissionID)
	}
	if sub.PullRequestID == 0 {
		return fmt.Errorf("submission has no pull request")
	}
	if _, err := s.pulls.AddPullRequestComment(ctx, sub.PullRequestID, body, doer); err != nil {
		return fmt.Errorf("add comment: %w", err)
	}
	return nil
}

// ResetApproval reverts the status from approved back to done. Grade and
// comment are preserved — the TA can either re-approve as-is or change the
// grade and approve again. Errors out if the submission is not currently
// approved.
func (s *service) ResetApproval(ctx context.Context, submissionID int64) error {
	return s.repo.ResetApproval(ctx, submissionID)
}
