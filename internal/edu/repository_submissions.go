package edu

import (
	"context"
	"fmt"
	"time"

	"forgejo.org/models/db"
)

func timeNowUnix() int64 {
	return time.Now().Unix()
}

func (r *xormRepository) CreateSubmission(ctx context.Context, s *Submission) error {
	_, err := db.GetEngine(ctx).Insert(s)
	if err != nil {
		return fmt.Errorf("insert submission: %w", err)
	}
	return nil
}

func (r *xormRepository) GetSubmission(ctx context.Context, assignmentID, userID int64) (*Submission, error) {
	s := &Submission{}
	has, err := db.GetEngine(ctx).Where("assignment_id = ? AND user_id = ?", assignmentID, userID).Get(s)
	if err != nil {
		return nil, fmt.Errorf("get submission: %w", err)
	}
	if !has {
		return nil, nil
	}
	return s, nil
}

func (r *xormRepository) GetSubmissionByID(ctx context.Context, id int64) (*Submission, error) {
	s := &Submission{}
	has, err := db.GetEngine(ctx).ID(id).Get(s)
	if err != nil {
		return nil, fmt.Errorf("get submission by id: %w", err)
	}
	if !has {
		return nil, nil
	}
	return s, nil
}

func (r *xormRepository) GetSubmissionByEnrollmentAssignment(ctx context.Context, enrollmentID, assignmentID int64) (*Submission, error) {
	s := &Submission{}
	has, err := db.GetEngine(ctx).Where("enrollment_id = ? AND assignment_id = ?", enrollmentID, assignmentID).Get(s)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return s, nil
}

func (r *xormRepository) UpdateSubmission(ctx context.Context, s *Submission) error {
	_, err := db.GetEngine(ctx).ID(s.ID).Cols("status", "updated_unix").Update(s)
	if err != nil {
		return fmt.Errorf("update submission: %w", err)
	}
	return nil
}

func (r *xormRepository) UpdateSubmissionPullRequestID(ctx context.Context, id, prID int64) error {
	now := timeNowUnix()
	_, err := db.GetEngine(ctx).ID(id).Cols("pull_request_id", "updated_unix").Update(&Submission{
		PullRequestID: prID,
		UpdatedUnix:   now,
	})
	if err != nil {
		return fmt.Errorf("update submission pr id: %w", err)
	}
	return nil
}

func (r *xormRepository) AutoGradeSubmission(ctx context.Context, submissionID int64, grade int) error {
	now := timeNowUnix()
	_, err := db.GetEngine(ctx).Where("id = ? AND manual_grade = ?", submissionID, false).
		Cols("grade", "status", "updated_unix").Update(&Submission{
		Grade:       grade,
		Status:      StatusSubmissionDone,
		UpdatedUnix: now,
	})
	if err != nil {
		return fmt.Errorf("auto-grade submission: %w", err)
	}
	return nil
}

// ApproveSubmission marks the submission as approved by an instructor:
// status=approved, manual_grade=true, fixates grade/comment/grader.
// Caller is expected to have validated the grade range (0..100) at the
// service level — repository performs the raw update.
func (r *xormRepository) ApproveSubmission(ctx context.Context, submissionID int64, grade int, comment string, gradedByID int64) error {
	now := timeNowUnix()
	_, err := db.GetEngine(ctx).ID(submissionID).Cols(
		"grade", "comment", "graded_by_id", "graded_unix",
		"status", "manual_grade", "updated_unix",
	).Update(&Submission{
		Grade:       grade,
		Comment:     comment,
		GradedByID:  gradedByID,
		GradedUnix:  now,
		Status:      StatusSubmissionApproved,
		ManualGrade: true,
		UpdatedUnix: now,
	})
	if err != nil {
		return fmt.Errorf("approve submission: %w", err)
	}
	return nil
}

// MarkSubmissionMerged transitions the submission status from approved to merged.
// The actual git-side merge is performed separately via PullRequestService.
func (r *xormRepository) MarkSubmissionMerged(ctx context.Context, submissionID int64) error {
	now := timeNowUnix()
	_, err := db.GetEngine(ctx).ID(submissionID).Cols("status", "updated_unix").Update(&Submission{
		Status:      StatusSubmissionMerged,
		UpdatedUnix: now,
	})
	if err != nil {
		return fmt.Errorf("mark submission merged: %w", err)
	}
	return nil
}

func (r *xormRepository) AddSubmissionComment(ctx context.Context, submissionID int64, body string, doerID int64) error {
	now := timeNowUnix()
	_, err := db.GetEngine(ctx).ID(submissionID).Cols("comment", "graded_by_id", "updated_unix").Update(&Submission{
		Comment:     body,
		GradedByID:  doerID,
		UpdatedUnix: now,
	})
	if err != nil {
		return fmt.Errorf("add submission comment: %w", err)
	}
	return nil
}

// ResetApproval reverts the submission status from approved back to done.
// The grade, comment, manual_grade flag and grader fields are preserved —
// the TA is expected to either re-approve with the same/updated grade or
// adjust the grade and approve again. Defensive WHERE on the status guards
// against accidental reset of a merged submission.
func (r *xormRepository) ResetApproval(ctx context.Context, submissionID int64) error {
	now := timeNowUnix()
	affected, err := db.GetEngine(ctx).
		Where("id = ? AND status = ?", submissionID, StatusSubmissionApproved).
		Cols("status", "updated_unix").
		Update(&Submission{
			Status:      StatusSubmissionDone,
			UpdatedUnix: now,
		})
	if err != nil {
		return fmt.Errorf("reset approval: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("submission %d is not in approved state", submissionID)
	}
	return nil
}
