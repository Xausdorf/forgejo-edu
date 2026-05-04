package edu

import (
	"context"
	"fmt"
	"time"

	"forgejo.org/models/db"
)

func (r *xormRepository) GetAssignmentsForUser(ctx context.Context, userID int64) ([]*Assignment, error) {
	var assignments []*Assignment
	err := db.GetEngine(ctx).
		Join("INNER", "edu_course_enrollments", "edu_course_enrollments.course_id = edu_assignments.course_id").
		Join("INNER", "edu_courses", "edu_courses.id = edu_assignments.course_id").
		Where("edu_course_enrollments.user_id = ?", userID).
		And("(edu_courses.end_unix = 0 OR edu_courses.end_unix > ?)", time.Now().Unix()).
		OrderBy("edu_assignments.created_unix DESC").
		Find(&assignments)
	if err != nil {
		return nil, fmt.Errorf("find assignments for user: %w", err)
	}
	return assignments, nil
}

func (r *xormRepository) UpdateAssignment(ctx context.Context, a *Assignment) error {
	_, err := db.GetEngine(ctx).ID(a.ID).Cols("title", "description", "allowed_files_glob", "deadline_unix", "updated_unix").Update(a)
	if err != nil {
		return fmt.Errorf("update assignment: %w", err)
	}
	return nil
}

func (r *xormRepository) DeleteAssignment(ctx context.Context, id int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		e := db.GetEngine(ctx)

		// 1. Find all submissions for this assignment
		var submissions []*Submission
		if err := e.Where("assignment_id = ?", id).Find(&submissions); err != nil {
			return fmt.Errorf("find submissions for assignment: %w", err)
		}

		// 2. Delete test results for all submissions
		for _, sub := range submissions {
			if _, err := e.Where("submission_id = ?", sub.ID).Delete(&TestResult{}); err != nil {
				return fmt.Errorf("delete test results for submission %d: %w", sub.ID, err)
			}
		}

		// 3. Delete submissions
		if _, err := e.Where("assignment_id = ?", id).Delete(&Submission{}); err != nil {
			return fmt.Errorf("delete submissions: %w", err)
		}

		// 4. Delete the assignment
		if _, err := e.ID(id).Delete(&Assignment{}); err != nil {
			return fmt.Errorf("delete assignment: %w", err)
		}

		return nil
	})
}
