package edu

import (
	"context"
	"fmt"
)

func (s *service) GetTestResults(ctx context.Context, submissionID int64) ([]*TestResult, error) {
	return s.repo.GetTestResultsBySubmission(ctx, submissionID)
}

func (s *service) GetLatestTestResult(ctx context.Context, submissionID int64) (*TestResult, error) {
	return s.repo.GetLatestTestResult(ctx, submissionID)
}

func (s *service) ApproveSubmission(ctx context.Context, submissionID int64, grade int, comment string, gradedByID int64) error {
	if grade < 0 || grade > 100 {
		return fmt.Errorf("grade must be between 0 and 100")
	}
	return s.repo.ApproveSubmission(ctx, submissionID, grade, comment, gradedByID)
}

func (s *service) MarkSubmissionMerged(ctx context.Context, submissionID int64) error {
	return s.repo.MarkSubmissionMerged(ctx, submissionID)
}

func (s *service) ResetApproval(ctx context.Context, submissionID int64) error {
	return s.repo.ResetApproval(ctx, submissionID)
}
