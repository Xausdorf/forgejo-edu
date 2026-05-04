package edu

import (
	"context"
	"fmt"

	"forgejo.org/models/db"
)

func (r *xormRepository) EnrollUser(ctx context.Context, enrollment *CourseEnrollment) error {
	_, err := db.GetEngine(ctx).Insert(enrollment)
	if err != nil {
		return fmt.Errorf("insert enrollment: %w", err)
	}
	return nil
}

func (r *xormRepository) GetEnrollment(ctx context.Context, courseID, userID int64) (*CourseEnrollment, error) {
	e := &CourseEnrollment{}
	has, err := db.GetEngine(ctx).Where("course_id = ? AND user_id = ?", courseID, userID).Get(e)
	if err != nil {
		return nil, fmt.Errorf("get enrollment: %w", err)
	}
	if !has {
		return nil, nil
	}
	return e, nil
}

func (r *xormRepository) GetEnrollmentByCourseUser(ctx context.Context, courseID, userID int64) (*CourseEnrollment, error) {
	e := &CourseEnrollment{}
	has, err := db.GetEngine(ctx).Where("course_id = ? AND user_id = ?", courseID, userID).Get(e)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return e, nil
}

func (r *xormRepository) GetEnrollments(ctx context.Context, courseID int64) ([]*CourseEnrollment, error) {
	var enrollments []*CourseEnrollment
	err := db.GetEngine(ctx).Where("course_id = ?", courseID).OrderBy("created_unix ASC").Find(&enrollments)
	if err != nil {
		return nil, fmt.Errorf("find enrollments: %w", err)
	}
	return enrollments, nil
}

func (r *xormRepository) RemoveEnrollment(ctx context.Context, courseID, userID int64) error {
	_, err := db.GetEngine(ctx).Where("course_id = ? AND user_id = ?", courseID, userID).Delete(&CourseEnrollment{})
	if err != nil {
		return fmt.Errorf("delete enrollment: %w", err)
	}
	return nil
}

func (r *xormRepository) UpdateEnrollment(ctx context.Context, e *CourseEnrollment) error {
	_, err := db.GetEngine(ctx).
		ID(e.ID).
		Cols("role", "group_name", "student_fork_repo_id").
		Update(e)
	if err != nil {
		return fmt.Errorf("update enrollment: %w", err)
	}
	return nil
}

func (r *xormRepository) GetEnrollmentByForkRepo(ctx context.Context, repoID int64) (*CourseEnrollment, error) {
	if repoID == 0 {
		return nil, nil
	}
	e := &CourseEnrollment{}
	has, err := db.GetEngine(ctx).Where("student_fork_repo_id = ?", repoID).Get(e)
	if err != nil {
		return nil, fmt.Errorf("get enrollment by fork repo: %w", err)
	}
	if !has {
		return nil, nil
	}
	return e, nil
}
