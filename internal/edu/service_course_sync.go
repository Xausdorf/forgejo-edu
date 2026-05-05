package edu

import (
	"context"
	"errors"
	"fmt"
	"time"

	"forgejo.org/models/db"
	issues_model "forgejo.org/models/issues"
	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/graceful"
	"forgejo.org/modules/log"
)

const (
	courseSyncSrcBranch  = "main"
	courseSyncDstBranch  = "course-sync"
	courseSyncMergeStyle = "merge"
)

// Sentinel errors for course sync.
var (
	ErrCourseSyncNoMaster     = errors.New("course has no tasks-master repository configured")
	ErrCourseSyncNoOrg        = errors.New("course is not bound to an organization")
	ErrCourseSyncNoInit       = errors.New("course init-forks task is not finished — run init forks before sync")
	ErrCourseSyncTaskNotFound = errors.New("course sync task not found")
	ErrCourseSyncPRNotFound   = errors.New("course sync PR not found")
	ErrCourseSyncPRNotPending = errors.New("course sync PR is not in pending state")
)

// StartCourseSync kicks off an asynchronous course-sync run: for each enrolled
// student with a fork, push tasks-master/main to a course-sync branch in the
// fork, ensure a PR course-sync → main exists, and try auto-merge.
//
// Returns the CourseSyncTask immediately; per-PR work runs in a goroutine.
// Conflicting and failed PRs are recorded but do not stop the run; the task
// finishes with StatusDone if at least one PR succeeded, otherwise StatusError.
func (s *service) StartCourseSync(ctx context.Context, courseID, doerID int64) (*CourseSyncTask, error) {
	if s.users == nil || s.pulls == nil {
		return nil, fmt.Errorf("user creator / pull request service not configured")
	}

	course, err := s.repo.GetCourseByID(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("get course: %w", err)
	}
	if course == nil {
		return nil, fmt.Errorf("course not found")
	}
	if course.OrgID == 0 {
		return nil, ErrCourseSyncNoOrg
	}
	if course.TasksMasterRepoID == 0 {
		return nil, ErrCourseSyncNoMaster
	}

	priorInit, err := s.repo.GetInitForksTaskByCourse(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("get init-forks task: %w", err)
	}
	if priorInit == nil || priorInit.Status != StatusDone {
		return nil, ErrCourseSyncNoInit
	}

	enrollments, err := s.repo.GetEnrollments(ctx, courseID)
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
	task := &CourseSyncTask{
		CourseID:    courseID,
		CreatorID:   doerID,
		TotalRepos:  len(students),
		Status:      StatusPending,
		CreatedUnix: now,
		UpdatedUnix: now,
	}
	if err := s.repo.CreateCourseSyncTask(ctx, task); err != nil {
		return nil, fmt.Errorf("create course sync task: %w", err)
	}

	if len(students) == 0 {
		task.Status = StatusDone
		task.UpdatedUnix = time.Now().Unix()
		if err := s.repo.UpdateCourseSyncTask(ctx, task); err != nil {
			log.Error("Failed to mark empty course-sync task done: %v", err)
		}
		return task, nil
	}

	go graceful.GetManager().RunWithShutdownContext(func(_ context.Context) {
		s.executeCourseSync(db.DefaultContext, task, course, doerID, students)
	})
	return task, nil
}

func (s *service) GetCourseSyncTaskByCourse(ctx context.Context, courseID int64) (*CourseSyncTask, error) {
	return s.repo.GetCourseSyncTaskByCourse(ctx, courseID)
}

func (s *service) GetCourseSyncTaskByID(ctx context.Context, id int64) (*CourseSyncTask, error) {
	return s.repo.GetCourseSyncTask(ctx, id)
}

func (s *service) ListCourseSyncPRsByTask(ctx context.Context, taskID int64) ([]*CourseSyncPR, error) {
	return s.repo.ListCourseSyncPRsByTask(ctx, taskID)
}

func (s *service) executeCourseSync(ctx context.Context, task *CourseSyncTask, course *Course, doerID int64, students []*CourseEnrollment) {
	task.Status = StatusRunning
	task.UpdatedUnix = time.Now().Unix()
	if err := s.repo.UpdateCourseSyncTask(ctx, task); err != nil {
		log.Error("Failed to mark course-sync task running: %v", err)
		return
	}

	doerUser, err := s.users.GetUserByName(ctx, systemMergeDoerName)
	if err != nil || doerUser == nil {
		s.failCourseSyncTask(ctx, task, fmt.Sprintf("get system doer: %v\n", err))
		return
	}

	for _, enrollment := range students {
		status, prErr := s.syncOneFork(ctx, task, course, doerUser, enrollment)
		switch status {
		case SyncPRStatusMerged:
			task.Synced++
		case SyncPRStatusConflict:
			task.Skipped++
		default:
			task.Failed++
			if prErr != nil {
				task.ErrorLog += fmt.Sprintf("enrollment %d: %v\n", enrollment.ID, prErr)
			}
		}
		task.UpdatedUnix = time.Now().Unix()
		if errUpd := s.repo.UpdateCourseSyncTask(ctx, task); errUpd != nil {
			log.Error("Failed to update course-sync task progress: %v", errUpd)
		}
	}

	if task.Failed > 0 && task.Synced == 0 && task.Skipped == 0 {
		task.Status = StatusError
	} else {
		task.Status = StatusDone
	}
	task.UpdatedUnix = time.Now().Unix()
	if err := s.repo.UpdateCourseSyncTask(ctx, task); err != nil {
		log.Error("Failed to finalize course-sync task: %v", err)
	}
}

// syncOneFork performs the per-student sync work and inserts a CourseSyncPR row.
// Returns the resulting SyncPRStatus and (for the failed/non-conflict path) the
// underlying error for inclusion in the task error log.
func (s *service) syncOneFork(ctx context.Context, task *CourseSyncTask, course *Course, doer *user_model.User, enrollment *CourseEnrollment) (SyncPRStatus, error) {
	syncPR := &CourseSyncPR{
		SyncTaskID:   task.ID,
		EnrollmentID: enrollment.ID,
		Status:       SyncPRStatusPending,
		CreatedUnix:  time.Now().Unix(),
		UpdatedUnix:  time.Now().Unix(),
	}
	if err := s.repo.CreateCourseSyncPR(ctx, syncPR); err != nil {
		return SyncPRStatusFailed, fmt.Errorf("create sync pr row: %w", err)
	}

	finish := func(status SyncPRStatus, errMsg string, prID int64) (SyncPRStatus, error) {
		syncPR.Status = status
		syncPR.ErrorMsg = errMsg
		syncPR.PullRequestID = prID
		syncPR.UpdatedUnix = time.Now().Unix()
		if err := s.repo.UpdateCourseSyncPR(ctx, syncPR); err != nil {
			log.Error("Failed to update course-sync pr %d: %v", syncPR.ID, err)
		}
		if status == SyncPRStatusFailed {
			return status, errors.New(errMsg)
		}
		return status, nil
	}

	forkRepo, err := s.forker.GetRepositoryByID(ctx, enrollment.StudentForkRepoID)
	if err != nil {
		return finish(SyncPRStatusFailed, fmt.Sprintf("load fork repo: %v", err), 0)
	}
	if forkRepo == nil {
		return finish(SyncPRStatusFailed, "fork repo not found", 0)
	}

	if err := s.forker.PushBranchToFork(ctx, doer, forkRepo, courseSyncSrcBranch, courseSyncDstBranch); err != nil {
		return finish(SyncPRStatusFailed, fmt.Sprintf("push %s -> %s: %v", courseSyncSrcBranch, courseSyncDstBranch, err), 0)
	}

	pr, err := s.ensureCourseSyncPullRequest(ctx, forkRepo, course, doer)
	if err != nil {
		return finish(SyncPRStatusFailed, fmt.Sprintf("ensure pr: %v", err), 0)
	}

	mergeErr := s.pulls.MergePullRequest(ctx, MergePullRequestOptions{
		PullRequestID: pr.ID,
		Doer:          doer,
		MergeStyle:    courseSyncMergeStyle,
		Message:       fmt.Sprintf("Course sync for %q", course.Name),
	})
	if mergeErr != nil {
		if s.pulls.IsMergeConflictError(mergeErr) {
			return finish(SyncPRStatusConflict, "merge conflict", pr.ID)
		}
		return finish(SyncPRStatusFailed, fmt.Sprintf("merge: %v", mergeErr), pr.ID)
	}
	return finish(SyncPRStatusMerged, "", pr.ID)
}

// ensureCourseSyncPullRequest returns the existing open course-sync → main PR
// in the fork, or creates a new one if none is open. Idempotent across re-runs:
// if the previous run left an open PR (e.g. because of a conflict), this run
// reuses it after the new force-push to course-sync moved its head ref.
func (s *service) ensureCourseSyncPullRequest(ctx context.Context, forkRepo *repo_model.Repository, course *Course, doer *user_model.User) (*issues_model.PullRequest, error) {
	existing, err := s.pulls.GetUnmergedPullRequest(ctx, forkRepo.ID, forkRepo.ID, courseSyncDstBranch, courseSyncSrcBranch)
	if err != nil {
		return nil, fmt.Errorf("lookup existing pr: %w", err)
	}
	if existing != nil {
		return existing, nil
	}
	pr, err := s.pulls.CreatePullRequest(ctx, CreatePullRequestOptions{
		BaseRepoID: forkRepo.ID,
		BaseBranch: courseSyncSrcBranch,
		HeadRepoID: forkRepo.ID,
		HeadBranch: courseSyncDstBranch,
		Title:      fmt.Sprintf("Course sync: %s", course.Name),
		Body:       fmt.Sprintf("Sync of tasks-master/%s into %s in course **%s**.", courseSyncSrcBranch, courseSyncDstBranch, course.Name),
		Doer:       doer,
	})
	if err != nil {
		return nil, fmt.Errorf("create pr: %w", err)
	}
	if pr == nil {
		return nil, fmt.Errorf("create pr returned nil")
	}
	return pr, nil
}

func (s *service) failCourseSyncTask(ctx context.Context, task *CourseSyncTask, msg string) {
	task.Status = StatusError
	task.ErrorLog += msg
	task.UpdatedUnix = time.Now().Unix()
	if err := s.repo.UpdateCourseSyncTask(ctx, task); err != nil {
		log.Error("Failed to mark course-sync task errored: %v", err)
	}
}

// MergeAllCourseSyncPRs walks all CourseSyncPR rows for taskID with status=pending
// (PR open, not yet merged) and tries to merge each one synchronously. Returns
// the number of successfully merged PRs. Conflicting PRs are flipped to status
// conflict; other errors flip to failed.
func (s *service) MergeAllCourseSyncPRs(ctx context.Context, taskID, doerID int64) (int, error) {
	if s.pulls == nil {
		return 0, fmt.Errorf("pull request service not configured")
	}
	task, err := s.repo.GetCourseSyncTask(ctx, taskID)
	if err != nil {
		return 0, fmt.Errorf("get task: %w", err)
	}
	if task == nil {
		return 0, ErrCourseSyncTaskNotFound
	}

	doer, err := s.users.GetUserByName(ctx, systemMergeDoerName)
	if err != nil || doer == nil {
		return 0, fmt.Errorf("get system doer: %w", err)
	}

	prs, err := s.repo.ListCourseSyncPRsByTask(ctx, taskID)
	if err != nil {
		return 0, fmt.Errorf("list prs: %w", err)
	}

	merged := 0
	for _, p := range prs {
		if p.Status != SyncPRStatusPending || p.PullRequestID == 0 {
			continue
		}
		if err := s.mergeOneSyncPR(ctx, p, doer); err != nil {
			log.Error("merge sync pr %d: %v", p.ID, err)
			continue
		}
		merged++
	}
	return merged, nil
}

// MergeCourseSyncPR merges a single CourseSyncPR by id. Validates that the row
// is in pending state and has a PR; flips status on the result.
func (s *service) MergeCourseSyncPR(ctx context.Context, syncPRID, doerID int64) error {
	if s.pulls == nil {
		return fmt.Errorf("pull request service not configured")
	}
	p, err := s.repo.GetCourseSyncPR(ctx, syncPRID)
	if err != nil {
		return fmt.Errorf("get sync pr: %w", err)
	}
	if p == nil {
		return ErrCourseSyncPRNotFound
	}
	if p.Status != SyncPRStatusPending {
		return ErrCourseSyncPRNotPending
	}
	if p.PullRequestID == 0 {
		return fmt.Errorf("sync pr %d has no pull request id", p.ID)
	}
	doer, err := s.users.GetUserByName(ctx, systemMergeDoerName)
	if err != nil || doer == nil {
		return fmt.Errorf("get system doer: %w", err)
	}
	return s.mergeOneSyncPR(ctx, p, doer)
}

func (s *service) mergeOneSyncPR(ctx context.Context, p *CourseSyncPR, doer *user_model.User) error {
	mergeErr := s.pulls.MergePullRequest(ctx, MergePullRequestOptions{
		PullRequestID: p.PullRequestID,
		Doer:          doer,
		MergeStyle:    courseSyncMergeStyle,
		Message:       fmt.Sprintf("Course sync merge (sync pr %d)", p.ID),
	})
	now := time.Now().Unix()
	switch {
	case mergeErr == nil:
		p.Status = SyncPRStatusMerged
		p.ErrorMsg = ""
	case s.pulls.IsMergeConflictError(mergeErr):
		p.Status = SyncPRStatusConflict
		p.ErrorMsg = "merge conflict"
	default:
		p.Status = SyncPRStatusFailed
		p.ErrorMsg = fmt.Sprintf("merge: %v", mergeErr)
	}
	p.UpdatedUnix = now
	if err := s.repo.UpdateCourseSyncPR(ctx, p); err != nil {
		log.Error("update sync pr %d: %v", p.ID, err)
	}
	return mergeErr
}
