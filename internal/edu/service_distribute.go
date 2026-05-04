package edu

import (
	"context"
	"errors"
	"fmt"
	"time"

	"forgejo.org/models/db"
	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/graceful"
	"forgejo.org/modules/log"
)

// Sentinel errors used by CreateAssignment and DistributeAssignment.
var (
	ErrAssignmentTaskNameInvalid = errors.New("task_name must match ^[a-z0-9_-]+$ and be 1-100 chars")
	ErrAssignmentTaskNameInUse   = errors.New("an assignment with this task_name already exists in this course")
	ErrAllowedFilesGlobRequired  = errors.New("allowed_files_glob is required")
	ErrSubmitsBranchNotFound     = errors.New("branch submits/<task_name> not found in tasks-master")
	ErrInitForksNotDone          = errors.New("course init-forks task is not finished — distribute requires forks to exist")
)

// DistributeAssignment starts an asynchronous bulk operation that pushes the
// branch `submits/<TaskName>` from the course's tasks-master into every student
// fork, and creates a placeholder Submission(pending) per enrollment.
//
// Idempotent: enrollments that already have a Submission for this assignment
// are skipped (no force-push, no overwrite of student work).
//
// Prerequisites: the course must have TasksMasterRepoID set, and a prior
// InitForksTask must have completed with StatusDone (so all student forks
// exist and have StudentForkRepoID populated on the enrollment).
func (s *service) DistributeAssignment(ctx context.Context, assignmentID, doerID int64) (*DistributeTask, error) {
	if s.users == nil {
		return nil, fmt.Errorf("user creator not configured")
	}

	assignment, err := s.repo.GetAssignmentByID(ctx, assignmentID)
	if err != nil {
		return nil, fmt.Errorf("get assignment: %w", err)
	}
	if assignment == nil {
		return nil, fmt.Errorf("assignment not found")
	}

	course, err := s.repo.GetCourseByID(ctx, assignment.CourseID)
	if err != nil {
		return nil, fmt.Errorf("get course: %w", err)
	}
	if course == nil {
		return nil, fmt.Errorf("course not found")
	}
	if course.TasksMasterRepoID == 0 {
		return nil, ErrTasksMasterRepoNotSet
	}

	priorInit, err := s.repo.GetInitForksTaskByCourse(ctx, assignment.CourseID)
	if err != nil {
		return nil, fmt.Errorf("get init-forks task: %w", err)
	}
	if priorInit == nil || priorInit.Status != StatusDone {
		return nil, ErrInitForksNotDone
	}

	enrollments, err := s.repo.GetEnrollments(ctx, assignment.CourseID)
	if err != nil {
		return nil, fmt.Errorf("get enrollments: %w", err)
	}

	var students []*CourseEnrollment
	for _, e := range enrollments {
		if e.Role == RoleStudent && e.StudentForkRepoID != 0 {
			students = append(students, e)
		}
	}

	now := time.Now().Unix()
	task := &DistributeTask{
		AssignmentID:     assignmentID,
		CreatorID:        doerID,
		TotalEnrollments: len(students),
		Status:           StatusPending,
		CreatedUnix:      now,
		UpdatedUnix:      now,
	}
	if err := s.repo.CreateDistributeTask(ctx, task); err != nil {
		return nil, fmt.Errorf("create distribute task: %w", err)
	}

	if len(students) == 0 {
		task.Status = StatusDone
		task.UpdatedUnix = time.Now().Unix()
		if err := s.repo.UpdateDistributeTask(ctx, task); err != nil {
			log.Error("Failed to mark empty distribute task done: %v", err)
		}
		return task, nil
	}

	go graceful.GetManager().RunWithShutdownContext(func(_ context.Context) {
		s.executeDistribute(db.DefaultContext, task, course, assignment, doerID, students)
	})

	return task, nil
}

func (s *service) GetDistributeTaskByAssignment(ctx context.Context, assignmentID int64) (*DistributeTask, error) {
	return s.repo.GetDistributeTaskByAssignment(ctx, assignmentID)
}

func (s *service) GetDistributeTaskByID(ctx context.Context, id int64) (*DistributeTask, error) {
	return s.repo.GetDistributeTask(ctx, id)
}

// executeDistribute is the goroutine body — pushes the branch and creates
// placeholder Submissions per student.
func (s *service) executeDistribute(ctx context.Context, task *DistributeTask, course *Course, assignment *Assignment, doerID int64, students []*CourseEnrollment) {
	task.Status = StatusRunning
	task.UpdatedUnix = time.Now().Unix()
	if err := s.repo.UpdateDistributeTask(ctx, task); err != nil {
		log.Error("Failed to mark distribute task running: %v", err)
		return
	}

	doerUser, err := s.users.GetUserByID(ctx, doerID)
	if err != nil {
		s.failDistributeTask(ctx, task, fmt.Sprintf("get doer: %v\n", err))
		return
	}

	branchName := "submits/" + assignment.TaskName

	for _, enrollment := range students {
		if err := s.distributeOne(ctx, course, assignment, branchName, doerUser, enrollment); err != nil {
			task.Failed++
			task.ErrorLog += err.Error() + "\n"
		} else {
			task.Pushed++
		}
		task.UpdatedUnix = time.Now().Unix()
		if errUpd := s.repo.UpdateDistributeTask(ctx, task); errUpd != nil {
			log.Error("Failed to update distribute task progress: %v", errUpd)
		}
	}

	if task.Failed > 0 {
		task.Status = StatusError
	} else {
		task.Status = StatusDone
	}
	task.UpdatedUnix = time.Now().Unix()
	if err := s.repo.UpdateDistributeTask(ctx, task); err != nil {
		log.Error("Failed to finalize distribute task: %v", err)
	}
}

// distributeOne is idempotent: skips enrollments that already have a Submission
// for this assignment (so re-running distribute does not force-push over student
// commits). For new enrollments it pushes submits/<task> from tasks-master into
// the student fork via SyncFork (which uses InternalPushingEnvironment to
// bypass branch protection), then inserts Submission(pending).
func (s *service) distributeOne(ctx context.Context, course *Course, assignment *Assignment, branchName string, doerUser *user_model.User, enrollment *CourseEnrollment) error {
	existingSub, err := s.repo.GetSubmissionByEnrollmentAssignment(ctx, enrollment.ID, assignment.ID)
	if err != nil {
		return fmt.Errorf("enrollment %d: check existing submission: %w", enrollment.ID, err)
	}
	if existingSub != nil {
		return nil
	}

	forkRepo, err := s.forker.GetRepositoryByID(ctx, enrollment.StudentForkRepoID)
	if err != nil {
		return fmt.Errorf("enrollment %d: load fork: %w", enrollment.ID, err)
	}
	if forkRepo == nil {
		return fmt.Errorf("enrollment %d: fork repo %d not found", enrollment.ID, enrollment.StudentForkRepoID)
	}

	if err := s.pushSubmitsBranch(ctx, doerUser, forkRepo, branchName); err != nil {
		return fmt.Errorf("enrollment %d: push %s: %w", enrollment.ID, branchName, err)
	}

	sub := &Submission{
		AssignmentID: assignment.ID,
		EnrollmentID: enrollment.ID,
		UserID:       enrollment.UserID,
		BranchName:   branchName,
		Status:       StatusSubmissionPending,
		Grade:        -1,
	}
	if err := s.repo.CreateSubmission(ctx, sub); err != nil {
		return fmt.Errorf("enrollment %d: create submission: %w", enrollment.ID, err)
	}
	return nil
}

// pushSubmitsBranch pushes branchName from the fork's BaseRepo (tasks-master)
// into the fork. Reuses RepoForker.SyncFork — it already does the right thing
// (git.Push with InternalPushingEnvironment). Kept as a separate method so
// tests can mock it cleanly through s.forker.SyncFork.
func (s *service) pushSubmitsBranch(ctx context.Context, doer *user_model.User, forkRepo *repo_model.Repository, branchName string) error {
	return s.forker.SyncFork(ctx, doer, forkRepo, branchName)
}

func (s *service) failDistributeTask(ctx context.Context, task *DistributeTask, msg string) {
	task.Status = StatusError
	task.ErrorLog += msg
	task.UpdatedUnix = time.Now().Unix()
	if err := s.repo.UpdateDistributeTask(ctx, task); err != nil {
		log.Error("Failed to mark distribute task errored: %v", err)
	}
}
