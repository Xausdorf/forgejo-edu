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

func (r *xormRepository) GradeSubmission(ctx context.Context, submissionID int64, grade int, comment string, gradedByID int64) error {
	now := timeNowUnix()
	_, err := db.GetEngine(ctx).ID(submissionID).Cols("grade", "comment", "graded_by_id", "graded_unix", "status", "manual_grade", "updated_unix").Update(&Submission{
		Grade:       grade,
		Comment:     comment,
		GradedByID:  gradedByID,
		GradedUnix:  now,
		Status:      StatusSubmissionDone,
		ManualGrade: true,
		UpdatedUnix: now,
	})
	if err != nil {
		return fmt.Errorf("grade submission: %w", err)
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

func (r *xormRepository) ResetToAutoGrade(ctx context.Context, submissionID int64, grade int) error {
	now := timeNowUnix()
	var status SubmissionStatus = StatusSubmissionDone
	if grade < 0 {
		status = StatusSubmissionPending
	}
	_, err := db.GetEngine(ctx).ID(submissionID).Cols("grade", "manual_grade", "status", "updated_unix").Update(&Submission{
		Grade:       grade,
		ManualGrade: false,
		Status:      status,
		UpdatedUnix: now,
	})
	if err != nil {
		return fmt.Errorf("reset to auto grade: %w", err)
	}
	return nil
}
