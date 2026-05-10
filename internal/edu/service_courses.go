package edu

import (
	"context"
	"errors"
	"fmt"
	"time"

	"forgejo.org/models/perm"
	"forgejo.org/modules/log"
)

// ErrUserAlreadyEnrolled is returned when trying to enroll a user who is already enrolled in the course.
var ErrUserAlreadyEnrolled = errors.New("user is already enrolled in this course")

func (s *service) CreateCourse(ctx context.Context, creatorID int64, opts CreateCourseOptions) (*Course, error) {
	if opts.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if opts.StartUnix != 0 && opts.EndUnix != 0 && opts.EndUnix <= opts.StartUnix {
		return nil, fmt.Errorf("end date must be after start date")
	}

	now := time.Now().Unix()
	course := &Course{
		Name:              opts.Name,
		Description:       opts.Description,
		CreatorID:         creatorID,
		OrgID:             opts.OrgID,
		TasksMasterRepoID: opts.TasksMasterRepoID,
		StartUnix:         opts.StartUnix,
		EndUnix:           opts.EndUnix,
		CreatedUnix:       now,
		UpdatedUnix:       now,
	}

	if err := s.repo.CreateCourse(ctx, course); err != nil {
		return nil, fmt.Errorf("create course: %w", err)
	}

	// Auto-enroll the creator as teacher
	enrollment := &CourseEnrollment{
		CourseID:    course.ID,
		UserID:      creatorID,
		Role:        RoleTeacher,
		CreatedUnix: now,
	}
	if err := s.repo.EnrollUser(ctx, enrollment); err != nil {
		return nil, fmt.Errorf("enroll creator: %w", err)
	}

	if err := s.addToOrgTeam(ctx, course.ID, creatorID, RoleTeacher); err != nil {
		log.Error("Failed to add creator %d to org team for course %d: %v", creatorID, course.ID, err)
	}

	return course, nil
}

func (s *service) GetCourseByID(ctx context.Context, id int64) (*Course, error) {
	return s.repo.GetCourseByID(ctx, id)
}

func (s *service) GetCoursesForUser(ctx context.Context, userID int64) ([]*Course, error) {
	return s.repo.GetCoursesByUser(ctx, userID)
}

func (s *service) UpdateCourse(ctx context.Context, course *Course) error {
	if course.Name == "" {
		return fmt.Errorf("name is required")
	}
	if course.StartUnix != 0 && course.EndUnix != 0 && course.EndUnix <= course.StartUnix {
		return fmt.Errorf("end date must be after start date")
	}

	course.UpdatedUnix = time.Now().Unix()
	return s.repo.UpdateCourse(ctx, course)
}

func (s *service) DeleteCourse(ctx context.Context, id int64) error {
	return s.repo.DeleteCourse(ctx, id)
}

func (s *service) EnrollUser(ctx context.Context, opts EnrollUserOptions) error {
	// Check if the user is already enrolled to avoid UNIQUE constraint violation
	existing, err := s.repo.GetEnrollment(ctx, opts.CourseID, opts.UserID)
	if err != nil {
		return fmt.Errorf("check existing enrollment: %w", err)
	}
	if existing != nil {
		return ErrUserAlreadyEnrolled
	}

	enrollment := &CourseEnrollment{
		CourseID:    opts.CourseID,
		UserID:      opts.UserID,
		Role:        opts.Role,
		GroupName:   opts.GroupName,
		CreatedUnix: time.Now().Unix(),
	}
	if err := s.repo.EnrollUser(ctx, enrollment); err != nil {
		return err
	}

	if err := s.addToOrgTeam(ctx, opts.CourseID, opts.UserID, opts.Role); err != nil {
		log.Error("Failed to add user %d to org team for course %d: %v", opts.UserID, opts.CourseID, err)
	}

	if err := s.ensureForkForEnrollment(ctx, opts.CourseID, opts.UserID); err != nil {
		log.Error("Failed to ensure fork for late enrollment (course %d, user %d): %v", opts.CourseID, opts.UserID, err)
	}

	return nil
}

func (s *service) GetEnrollments(ctx context.Context, courseID int64) ([]*CourseEnrollment, error) {
	return s.repo.GetEnrollments(ctx, courseID)
}

func (s *service) RemoveEnrollment(ctx context.Context, courseID, userID int64) error {
	// Capture fork id before deletion so we can remove the collaborator afterwards.
	var formerForkRepoID int64
	if existing, err := s.repo.GetEnrollment(ctx, courseID, userID); err == nil && existing != nil {
		formerForkRepoID = existing.StudentForkRepoID
	}

	if err := s.repo.RemoveEnrollment(ctx, courseID, userID); err != nil {
		return err
	}

	if err := s.removeFromOrgTeam(ctx, courseID, userID); err != nil {
		log.Error("Failed to remove user %d from org team for course %d: %v", userID, courseID, err)
	}

	if err := s.ensureCollaboratorRemovedForEnrollment(ctx, courseID, userID, formerForkRepoID); err != nil {
		log.Error("Failed to remove collaborator (course %d, user %d, fork %d): %v", courseID, userID, formerForkRepoID, err)
	}

	return nil
}

func (s *service) GetAssignmentsByCourse(ctx context.Context, courseID int64) ([]*Assignment, error) {
	return s.repo.GetAssignmentsByCourse(ctx, courseID)
}

func eduTeamName(courseID int64, role RoleType) string {
	switch role {
	case RoleTeacher, RoleAdmin:
		return fmt.Sprintf("edu-course-%d-teachers", courseID)
	case RoleTA:
		return fmt.Sprintf("edu-course-%d-ta", courseID)
	default:
		return fmt.Sprintf("edu-course-%d-students", courseID)
	}
}

func eduAccessMode(role RoleType) perm.AccessMode {
	switch role {
	case RoleTeacher, RoleAdmin:
		return perm.AccessModeAdmin
	default:
		// Both TA and student get Read on the team.
		return perm.AccessModeRead
	}
}

// addToOrgTeam ensures the user is in the right org team for the course.
// If the course has no OrgID or the org manager is not configured, this is a no-op.
func (s *service) addToOrgTeam(ctx context.Context, courseID, userID int64, role RoleType) error {
	if s.orgs == nil {
		return nil
	}

	course, err := s.repo.GetCourseByID(ctx, courseID)
	if err != nil {
		return fmt.Errorf("get course for team mapping: %w", err)
	}
	if course == nil || course.OrgID == 0 {
		return nil
	}

	teamName := eduTeamName(courseID, role)
	accessMode := eduAccessMode(role)
	includesAllRepositories := role != RoleStudent

	team, err := s.orgs.EnsureTeam(ctx, course.OrgID, teamName, accessMode, includesAllRepositories)
	if err != nil {
		return fmt.Errorf("ensure team %s: %w", teamName, err)
	}

	if err := s.orgs.AddTeamMember(ctx, team, userID); err != nil {
		return fmt.Errorf("add team member: %w", err)
	}

	if role == RoleStudent && course.TasksMasterRepoID != 0 {
		if err := s.orgs.AddTeamRepositoryIfMissing(ctx, team, course.TasksMasterRepoID); err != nil {
			return fmt.Errorf("attach tasks-master to students team: %w", err)
		}
	}

	return nil
}

// removeFromOrgTeam removes a user from the org teams for the course.
func (s *service) removeFromOrgTeam(ctx context.Context, courseID, userID int64) error {
	if s.orgs == nil {
		return nil
	}

	course, err := s.repo.GetCourseByID(ctx, courseID)
	if err != nil {
		return fmt.Errorf("get course for team removal: %w", err)
	}
	if course == nil || course.OrgID == 0 {
		return nil
	}

	// Try to remove from all team types
	for _, role := range []RoleType{RoleStudent, RoleTA, RoleTeacher} {
		teamName := eduTeamName(courseID, role)
		team, err := s.orgs.GetTeam(ctx, course.OrgID, teamName)
		if err != nil {
			continue // team doesn't exist, nothing to remove from
		}
		if err := s.orgs.RemoveTeamMember(ctx, team, userID); err != nil {
			log.Error("Failed to remove user %d from team %s: %v", userID, teamName, err)
		}
	}

	return nil
}
