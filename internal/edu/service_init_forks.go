package edu

import (
	"context"
	"errors"
	"fmt"
	"time"

	"forgejo.org/models/db"
	"forgejo.org/models/perm"
	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/graceful"
	"forgejo.org/modules/log"
)

// ErrTasksMasterRepoNotSet is returned when init-forks is attempted on a course that has no tasks-master repo bound.
var ErrTasksMasterRepoNotSet = errors.New("course has no tasks-master repository configured")

// ErrCourseHasNoOrg is returned when init-forks is attempted on a course without an org binding.
var ErrCourseHasNoOrg = errors.New("course is not bound to an organization")

// InitCourseForks starts an asynchronous bulk operation that creates a tasks-master fork
// for every student enrolled in the course (if not already created), adds the student as a
// collaborator with Write access on their fork, and protects the main branch.
//
// Idempotent: students whose enrollment already has StudentForkRepoID != 0 are skipped.
// Returns the InitForksTask immediately; the operation runs in a background goroutine.
func (s *service) InitCourseForks(ctx context.Context, courseID, doerID int64) (*InitForksTask, error) {
	if s.users == nil {
		return nil, fmt.Errorf("user creator not configured")
	}

	course, err := s.repo.GetCourseByID(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("get course: %w", err)
	}
	if course == nil {
		return nil, fmt.Errorf("course not found")
	}
	if course.OrgID == 0 {
		return nil, ErrCourseHasNoOrg
	}
	if course.TasksMasterRepoID == 0 {
		return nil, ErrTasksMasterRepoNotSet
	}

	baseRepo, err := s.forker.GetRepositoryByID(ctx, course.TasksMasterRepoID)
	if err != nil {
		return nil, fmt.Errorf("get tasks-master repo: %w", err)
	}
	if baseRepo == nil {
		return nil, fmt.Errorf("tasks-master repo not found")
	}

	enrollments, err := s.repo.GetEnrollments(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("get enrollments: %w", err)
	}

	var students []*CourseEnrollment
	for _, e := range enrollments {
		if e.Role == RoleStudent {
			students = append(students, e)
		}
	}

	now := time.Now().Unix()
	task := &InitForksTask{
		CourseID:    courseID,
		CreatorID:   doerID,
		TotalUsers:  len(students),
		Status:      StatusPending,
		CreatedUnix: now,
		UpdatedUnix: now,
	}
	if err := s.repo.CreateInitForksTask(ctx, task); err != nil {
		return nil, fmt.Errorf("create init forks task: %w", err)
	}

	if len(students) == 0 {
		task.Status = StatusDone
		task.UpdatedUnix = time.Now().Unix()
		if err := s.repo.UpdateInitForksTask(ctx, task); err != nil {
			log.Error("Failed to mark empty init-forks task done: %v", err)
		}
		return task, nil
	}

	if s.orgs != nil {
		studentsTeam, err := s.orgs.GetTeam(ctx, course.OrgID, eduTeamName(courseID, RoleStudent))
		if err == nil && studentsTeam != nil {
			if err := s.orgs.AddTeamRepositoryIfMissing(ctx, studentsTeam, course.TasksMasterRepoID); err != nil {
				log.Error("attach tasks-master to students team for course %d: %v", courseID, err)
			}
		}
	}

	go graceful.GetManager().RunWithShutdownContext(func(_ context.Context) {
		s.executeInitForks(db.DefaultContext, task, course, baseRepo, doerID, students)
	})

	return task, nil
}

func (s *service) GetInitForksTaskByCourse(ctx context.Context, courseID int64) (*InitForksTask, error) {
	return s.repo.GetInitForksTaskByCourse(ctx, courseID)
}

func (s *service) GetInitForksTaskByID(ctx context.Context, id int64) (*InitForksTask, error) {
	return s.repo.GetInitForksTask(ctx, id)
}

// executeInitForks runs the actual fork loop in a background goroutine.
func (s *service) executeInitForks(ctx context.Context, task *InitForksTask, course *Course, baseRepo *repo_model.Repository, doerID int64, students []*CourseEnrollment) {
	task.Status = StatusRunning
	task.UpdatedUnix = time.Now().Unix()
	if err := s.repo.UpdateInitForksTask(ctx, task); err != nil {
		log.Error("Failed to mark init-forks task running: %v", err)
		return
	}

	doerUser, err := s.users.GetUserByID(ctx, doerID)
	if err != nil {
		s.failInitTask(ctx, task, fmt.Sprintf("get doer: %v\n", err))
		return
	}

	orgUser, err := s.users.GetUserByID(ctx, course.OrgID)
	if err != nil {
		s.failInitTask(ctx, task, fmt.Sprintf("get org user: %v\n", err))
		return
	}

	for _, enrollment := range students {
		if err := s.initOneFork(ctx, course, baseRepo, doerUser, orgUser, enrollment); err != nil {
			task.Failed++
			task.ErrorLog += err.Error() + "\n"
		} else {
			task.Completed++
		}
		task.UpdatedUnix = time.Now().Unix()
		if errUpd := s.repo.UpdateInitForksTask(ctx, task); errUpd != nil {
			log.Error("Failed to update init-forks task progress: %v", errUpd)
		}
	}

	if task.Failed > 0 {
		task.Status = StatusError
	} else {
		task.Status = StatusDone
	}
	task.UpdatedUnix = time.Now().Unix()
	if err := s.repo.UpdateInitForksTask(ctx, task); err != nil {
		log.Error("Failed to finalize init-forks task: %v", err)
	}
}

// initOneFork performs the per-student work: fork creation (if missing), collaborator add, branch protection.
// All steps are idempotent. enrollment.StudentForkRepoID is updated on success.
func (s *service) initOneFork(ctx context.Context, course *Course, baseRepo *repo_model.Repository, doerUser, orgUser *user_model.User, enrollment *CourseEnrollment) error {
	studentUser, err := s.users.GetUserByID(ctx, enrollment.UserID)
	if err != nil {
		return fmt.Errorf("user %d: get user: %w", enrollment.UserID, err)
	}

	forkName := fmt.Sprintf("%s-tasks", studentUser.Name)

	var forkRepo *repo_model.Repository

	if enrollment.StudentForkRepoID != 0 {
		forkRepo, err = s.forker.GetRepositoryByID(ctx, enrollment.StudentForkRepoID)
		if err != nil {
			return fmt.Errorf("%s: load existing fork: %w", studentUser.Name, err)
		}
	} else {
		// Check if a repo with that name already exists in the org (e.g. orphaned from earlier run).
		existing, err := s.forker.GetRepositoryByOwnerAndName(ctx, course.OrgID, forkName)
		if err != nil {
			return fmt.Errorf("%s: lookup existing repo: %w", studentUser.Name, err)
		}
		if existing != nil {
			forkRepo = existing
		} else {
			forkRepo, err = s.forker.ForkRepositoryAndUpdates(ctx, doerUser, orgUser, ForkRepoOptions{
				BaseRepo: baseRepo,
				Name:     forkName,
			})
			if err != nil {
				return fmt.Errorf("%s: fork: %w", studentUser.Name, err)
			}
		}
		enrollment.StudentForkRepoID = forkRepo.ID
		if err := s.repo.UpdateEnrollment(ctx, enrollment); err != nil {
			return fmt.Errorf("%s: save fork id: %w", studentUser.Name, err)
		}
	}

	if err := s.forker.SetRepositoryPrivate(ctx, forkRepo.ID, true); err != nil {
		return fmt.Errorf("%s: make fork private: %w", studentUser.Name, err)
	}

	if err := s.forker.EnableActionsUnit(ctx, forkRepo.ID); err != nil {
		return fmt.Errorf("%s: enable actions: %w", studentUser.Name, err)
	}

	if err := s.forker.AddCollaborator(ctx, forkRepo.ID, studentUser.ID, perm.AccessModeWrite); err != nil {
		return fmt.Errorf("%s: add collaborator: %w", studentUser.Name, err)
	}

	branch, err := s.forker.GetDefaultBranch(ctx, forkRepo.ID)
	if err != nil {
		return fmt.Errorf("%s: get default branch: %w", studentUser.Name, err)
	}
	if branch == "" {
		branch = "main"
	}
	if err := s.forker.ProtectMainBranch(ctx, forkRepo.ID, branch); err != nil {
		return fmt.Errorf("%s: protect branch: %w", studentUser.Name, err)
	}

	return nil
}

func (s *service) failInitTask(ctx context.Context, task *InitForksTask, msg string) {
	task.Status = StatusError
	task.ErrorLog += msg
	task.UpdatedUnix = time.Now().Unix()
	if err := s.repo.UpdateInitForksTask(ctx, task); err != nil {
		log.Error("Failed to mark init-forks task errored: %v", err)
	}
}

// ensureForkForEnrollment is the lazy-init counterpart of InitCourseForks for a single
// enrollment. Called from EnrollUser after a successful enrollment. No-op if the course has
// no tasks-master, no org, or if the previous course-wide InitForksTask never ran (in which
// case the teacher will re-run it explicitly).
func (s *service) ensureForkForEnrollment(ctx context.Context, courseID, userID int64) error {
	if s.users == nil {
		return nil
	}
	course, err := s.repo.GetCourseByID(ctx, courseID)
	if err != nil {
		return fmt.Errorf("get course: %w", err)
	}
	if course == nil || course.OrgID == 0 || course.TasksMasterRepoID == 0 {
		return nil
	}

	priorTask, err := s.repo.GetInitForksTaskByCourse(ctx, courseID)
	if err != nil {
		return fmt.Errorf("get prior init task: %w", err)
	}
	if priorTask == nil || priorTask.Status != StatusDone {
		// No prior init has finished — defer to the next explicit "Init forks" click.
		return nil
	}

	enrollment, err := s.repo.GetEnrollmentByCourseUser(ctx, courseID, userID)
	if err != nil {
		return fmt.Errorf("get enrollment: %w", err)
	}
	if enrollment == nil || enrollment.Role != RoleStudent {
		return nil
	}
	if enrollment.StudentForkRepoID != 0 {
		return nil
	}

	baseRepo, err := s.forker.GetRepositoryByID(ctx, course.TasksMasterRepoID)
	if err != nil {
		return fmt.Errorf("get tasks-master: %w", err)
	}
	if baseRepo == nil {
		return fmt.Errorf("tasks-master not found")
	}

	doerUser, err := s.users.GetUserByID(ctx, course.CreatorID)
	if err != nil {
		return fmt.Errorf("get course creator: %w", err)
	}
	orgUser, err := s.users.GetUserByID(ctx, course.OrgID)
	if err != nil {
		return fmt.Errorf("get org user: %w", err)
	}

	return s.initOneFork(ctx, course, baseRepo, doerUser, orgUser, enrollment)
}

// ensureCollaboratorRemovedForEnrollment is the unenroll-time counterpart: drops the
// student's collaborator entry from their fork. The fork itself is left in the org as
// "orphaned" per spec section 2.
func (s *service) ensureCollaboratorRemovedForEnrollment(ctx context.Context, courseID, userID int64, formerForkRepoID int64) error {
	if formerForkRepoID == 0 {
		return nil
	}
	return s.forker.RemoveCollaborator(ctx, formerForkRepoID, userID)
}
