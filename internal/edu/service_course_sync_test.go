package edu

import (
	"context"
	"errors"
	"testing"

	issues_model "forgejo.org/models/issues"
	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newCourseSyncService() (*service, *MockRepository, *MockRepoForker, *MockUserCreator, *MockPullRequestService) {
	mr := new(MockRepository)
	mf := new(MockRepoForker)
	mu := new(MockUserCreator)
	mp := new(MockPullRequestService)
	s := &service{repo: mr, forker: mf, users: mu, pulls: mp}
	return s, mr, mf, mu, mp
}

// ---------------------- StartCourseSync validations ----------------------

func TestStartCourseSync_RequiresOrg(t *testing.T) {
	s, mr, _, _, _ := newCourseSyncService()
	ctx := context.Background()
	mr.On("GetCourseByID", ctx, int64(1)).Return(&Course{ID: 1, OrgID: 0, TasksMasterRepoID: 99}, nil)

	_, err := s.StartCourseSync(ctx, 1, 5)
	assert.ErrorIs(t, err, ErrCourseSyncNoOrg)
}

func TestStartCourseSync_RequiresMaster(t *testing.T) {
	s, mr, _, _, _ := newCourseSyncService()
	ctx := context.Background()
	mr.On("GetCourseByID", ctx, int64(1)).Return(&Course{ID: 1, OrgID: 5, TasksMasterRepoID: 0}, nil)

	_, err := s.StartCourseSync(ctx, 1, 5)
	assert.ErrorIs(t, err, ErrCourseSyncNoMaster)
}

func TestStartCourseSync_RequiresInitForksDone(t *testing.T) {
	s, mr, _, _, _ := newCourseSyncService()
	ctx := context.Background()
	mr.On("GetCourseByID", ctx, int64(1)).Return(&Course{ID: 1, OrgID: 5, TasksMasterRepoID: 99}, nil)
	mr.On("GetInitForksTaskByCourse", ctx, int64(1)).Return(nil, nil)

	_, err := s.StartCourseSync(ctx, 1, 5)
	assert.ErrorIs(t, err, ErrCourseSyncNoInit)
}

func TestStartCourseSync_RequiresInitForksStatusDone(t *testing.T) {
	s, mr, _, _, _ := newCourseSyncService()
	ctx := context.Background()
	mr.On("GetCourseByID", ctx, int64(1)).Return(&Course{ID: 1, OrgID: 5, TasksMasterRepoID: 99}, nil)
	mr.On("GetInitForksTaskByCourse", ctx, int64(1)).Return(&InitForksTask{Status: StatusError}, nil)

	_, err := s.StartCourseSync(ctx, 1, 5)
	assert.ErrorIs(t, err, ErrCourseSyncNoInit)
}

func TestStartCourseSync_NoStudentsMarksDone(t *testing.T) {
	s, mr, _, _, _ := newCourseSyncService()
	ctx := context.Background()
	mr.On("GetCourseByID", ctx, int64(1)).Return(&Course{ID: 1, OrgID: 5, TasksMasterRepoID: 99}, nil)
	mr.On("GetInitForksTaskByCourse", ctx, int64(1)).Return(&InitForksTask{Status: StatusDone}, nil)
	mr.On("GetEnrollments", ctx, int64(1)).Return([]*CourseEnrollment{
		{ID: 1, Role: RoleTeacher, StudentForkRepoID: 0},
	}, nil)
	mr.On("CreateCourseSyncTask", ctx, mock.MatchedBy(func(t *CourseSyncTask) bool {
		return t.CourseID == 1 && t.TotalRepos == 0
	})).Return(nil)
	mr.On("UpdateCourseSyncTask", ctx, mock.MatchedBy(func(t *CourseSyncTask) bool {
		return t.Status == StatusDone
	})).Return(nil)

	task, err := s.StartCourseSync(ctx, 1, 5)
	assert.NoError(t, err)
	assert.Equal(t, StatusDone, task.Status)
	mr.AssertExpectations(t)
}

func TestStartCourseSync_SkipsStudentsWithoutFork(t *testing.T) {
	s, mr, _, _, _ := newCourseSyncService()
	ctx := context.Background()
	mr.On("GetCourseByID", ctx, int64(1)).Return(&Course{ID: 1, OrgID: 5, TasksMasterRepoID: 99}, nil)
	mr.On("GetInitForksTaskByCourse", ctx, int64(1)).Return(&InitForksTask{Status: StatusDone}, nil)
	mr.On("GetEnrollments", ctx, int64(1)).Return([]*CourseEnrollment{
		{ID: 1, Role: RoleStudent, StudentForkRepoID: 0},
		{ID: 2, Role: RoleStudent, StudentForkRepoID: 0},
	}, nil)
	mr.On("CreateCourseSyncTask", ctx, mock.MatchedBy(func(t *CourseSyncTask) bool {
		return t.CourseID == 1 && t.TotalRepos == 0
	})).Return(nil)
	mr.On("UpdateCourseSyncTask", ctx, mock.Anything).Return(nil)

	_, err := s.StartCourseSync(ctx, 1, 5)
	assert.NoError(t, err)
}

// ---------------------- syncOneFork classification ----------------------

func newSyncFixture() (*Course, *user_model.User, *CourseEnrollment, *repo_model.Repository) {
	course := &Course{ID: 1, OrgID: 5, TasksMasterRepoID: 99, Name: "C++ Programming"}
	doer := &user_model.User{ID: 99, Name: "eduadmin"}
	enr := &CourseEnrollment{ID: 7, CourseID: 1, UserID: 42, Role: RoleStudent, StudentForkRepoID: 555}
	fork := &repo_model.Repository{ID: 555, Name: "alice-tasks"}
	return course, doer, enr, fork
}

func TestSyncOneFork_HappyPathMerged(t *testing.T) {
	s, mr, mf, _, mp := newCourseSyncService()
	ctx := context.Background()
	course, doer, enr, fork := newSyncFixture()
	task := &CourseSyncTask{ID: 11}
	pr := &issues_model.PullRequest{ID: 222}

	mr.On("CreateCourseSyncPR", ctx, mock.MatchedBy(func(p *CourseSyncPR) bool {
		return p.SyncTaskID == 11 && p.EnrollmentID == 7 && p.Status == SyncPRStatusPending
	})).Return(nil).Run(func(args mock.Arguments) {
		args.Get(1).(*CourseSyncPR).ID = 33
	})
	mf.On("GetRepositoryByID", ctx, int64(555)).Return(fork, nil)
	mf.On("PushBranchToFork", ctx, doer, fork, "main", "course-sync").Return(nil)
	mp.On("GetUnmergedPullRequest", ctx, int64(555), int64(555), "course-sync", "main").Return(nil, nil)
	mp.On("CreatePullRequest", ctx, mock.MatchedBy(func(opts CreatePullRequestOptions) bool {
		return opts.BaseRepoID == 555 && opts.HeadRepoID == 555 &&
			opts.HeadBranch == "course-sync" && opts.BaseBranch == "main"
	})).Return(pr, nil)
	mp.On("MergePullRequest", ctx, mock.MatchedBy(func(opts MergePullRequestOptions) bool {
		return opts.PullRequestID == 222 && opts.Doer == doer && opts.MergeStyle == "merge"
	})).Return(nil)
	mr.On("UpdateCourseSyncPR", ctx, mock.MatchedBy(func(p *CourseSyncPR) bool {
		return p.Status == SyncPRStatusMerged && p.PullRequestID == 222
	})).Return(nil)

	status, err := s.syncOneFork(ctx, task, course, doer, enr)
	assert.NoError(t, err)
	assert.Equal(t, SyncPRStatusMerged, status)
}

func TestSyncOneFork_ReusesExistingOpenPR(t *testing.T) {
	s, mr, mf, _, mp := newCourseSyncService()
	ctx := context.Background()
	course, doer, enr, fork := newSyncFixture()
	task := &CourseSyncTask{ID: 11}
	existingPR := &issues_model.PullRequest{ID: 333}

	mr.On("CreateCourseSyncPR", ctx, mock.Anything).Return(nil)
	mf.On("GetRepositoryByID", ctx, int64(555)).Return(fork, nil)
	mf.On("PushBranchToFork", ctx, doer, fork, "main", "course-sync").Return(nil)
	mp.On("GetUnmergedPullRequest", ctx, int64(555), int64(555), "course-sync", "main").Return(existingPR, nil)
	mp.On("MergePullRequest", ctx, mock.MatchedBy(func(opts MergePullRequestOptions) bool {
		return opts.PullRequestID == 333
	})).Return(nil)
	mr.On("UpdateCourseSyncPR", ctx, mock.MatchedBy(func(p *CourseSyncPR) bool {
		return p.Status == SyncPRStatusMerged && p.PullRequestID == 333
	})).Return(nil)

	status, err := s.syncOneFork(ctx, task, course, doer, enr)
	assert.NoError(t, err)
	assert.Equal(t, SyncPRStatusMerged, status)
	mp.AssertNotCalled(t, "CreatePullRequest", mock.Anything, mock.Anything)
}

func TestSyncOneFork_MergeConflict(t *testing.T) {
	s, mr, mf, _, mp := newCourseSyncService()
	ctx := context.Background()
	course, doer, enr, fork := newSyncFixture()
	task := &CourseSyncTask{ID: 11}
	pr := &issues_model.PullRequest{ID: 222}
	conflictErr := errors.New("merge conflict")

	mr.On("CreateCourseSyncPR", ctx, mock.Anything).Return(nil)
	mf.On("GetRepositoryByID", ctx, int64(555)).Return(fork, nil)
	mf.On("PushBranchToFork", ctx, doer, fork, "main", "course-sync").Return(nil)
	mp.On("GetUnmergedPullRequest", ctx, int64(555), int64(555), "course-sync", "main").Return(nil, nil)
	mp.On("CreatePullRequest", ctx, mock.Anything).Return(pr, nil)
	mp.On("MergePullRequest", ctx, mock.Anything).Return(conflictErr)
	mp.On("IsMergeConflictError", conflictErr).Return(true)
	mr.On("UpdateCourseSyncPR", ctx, mock.MatchedBy(func(p *CourseSyncPR) bool {
		return p.Status == SyncPRStatusConflict && p.PullRequestID == 222
	})).Return(nil)

	status, err := s.syncOneFork(ctx, task, course, doer, enr)
	assert.NoError(t, err)
	assert.Equal(t, SyncPRStatusConflict, status)
}

func TestSyncOneFork_PushFails(t *testing.T) {
	s, mr, mf, _, _ := newCourseSyncService()
	ctx := context.Background()
	course, doer, enr, fork := newSyncFixture()
	task := &CourseSyncTask{ID: 11}

	mr.On("CreateCourseSyncPR", ctx, mock.Anything).Return(nil)
	mf.On("GetRepositoryByID", ctx, int64(555)).Return(fork, nil)
	mf.On("PushBranchToFork", ctx, doer, fork, "main", "course-sync").Return(errors.New("network down"))
	mr.On("UpdateCourseSyncPR", ctx, mock.MatchedBy(func(p *CourseSyncPR) bool {
		return p.Status == SyncPRStatusFailed && p.PullRequestID == 0
	})).Return(nil)

	status, err := s.syncOneFork(ctx, task, course, doer, enr)
	assert.Error(t, err)
	assert.Equal(t, SyncPRStatusFailed, status)
}

func TestSyncOneFork_ForkRepoNotFound(t *testing.T) {
	s, mr, mf, _, _ := newCourseSyncService()
	ctx := context.Background()
	course, doer, enr, _ := newSyncFixture()
	task := &CourseSyncTask{ID: 11}

	mr.On("CreateCourseSyncPR", ctx, mock.Anything).Return(nil)
	mf.On("GetRepositoryByID", ctx, int64(555)).Return(nil, nil)
	mr.On("UpdateCourseSyncPR", ctx, mock.MatchedBy(func(p *CourseSyncPR) bool {
		return p.Status == SyncPRStatusFailed
	})).Return(nil)

	status, _ := s.syncOneFork(ctx, task, course, doer, enr)
	assert.Equal(t, SyncPRStatusFailed, status)
}

// ---------------------- MergeAll / MergeOne ----------------------

func TestMergeAllCourseSyncPRs_SkipsNonPending(t *testing.T) {
	s, mr, _, mu, mp := newCourseSyncService()
	ctx := context.Background()
	doer := &user_model.User{ID: 99, Name: "eduadmin"}

	mr.On("GetCourseSyncTask", ctx, int64(11)).Return(&CourseSyncTask{ID: 11}, nil)
	mu.On("GetUserByName", ctx, "eduadmin").Return(doer, nil)
	mr.On("ListCourseSyncPRsByTask", ctx, int64(11)).Return([]*CourseSyncPR{
		{ID: 1, Status: SyncPRStatusPending, PullRequestID: 100},
		{ID: 2, Status: SyncPRStatusMerged, PullRequestID: 101},
		{ID: 3, Status: SyncPRStatusConflict, PullRequestID: 102},
		{ID: 4, Status: SyncPRStatusPending, PullRequestID: 0},
	}, nil)
	mp.On("MergePullRequest", ctx, mock.MatchedBy(func(opts MergePullRequestOptions) bool {
		return opts.PullRequestID == 100
	})).Return(nil)
	mr.On("UpdateCourseSyncPR", ctx, mock.MatchedBy(func(p *CourseSyncPR) bool {
		return p.ID == 1 && p.Status == SyncPRStatusMerged
	})).Return(nil)

	merged, err := s.MergeAllCourseSyncPRs(ctx, 11, 5)
	assert.NoError(t, err)
	assert.Equal(t, 1, merged)
	mp.AssertNumberOfCalls(t, "MergePullRequest", 1)
}

func TestMergeAllCourseSyncPRs_TaskNotFound(t *testing.T) {
	s, mr, _, _, _ := newCourseSyncService()
	ctx := context.Background()
	mr.On("GetCourseSyncTask", ctx, int64(11)).Return(nil, nil)

	_, err := s.MergeAllCourseSyncPRs(ctx, 11, 5)
	assert.ErrorIs(t, err, ErrCourseSyncTaskNotFound)
}

func TestMergeCourseSyncPR_HappyPath(t *testing.T) {
	s, mr, _, mu, mp := newCourseSyncService()
	ctx := context.Background()
	doer := &user_model.User{ID: 99, Name: "eduadmin"}

	mr.On("GetCourseSyncPR", ctx, int64(33)).Return(&CourseSyncPR{
		ID: 33, Status: SyncPRStatusPending, PullRequestID: 100,
	}, nil)
	mu.On("GetUserByName", ctx, "eduadmin").Return(doer, nil)
	mp.On("MergePullRequest", ctx, mock.MatchedBy(func(opts MergePullRequestOptions) bool {
		return opts.PullRequestID == 100
	})).Return(nil)
	mr.On("UpdateCourseSyncPR", ctx, mock.MatchedBy(func(p *CourseSyncPR) bool {
		return p.ID == 33 && p.Status == SyncPRStatusMerged
	})).Return(nil)

	err := s.MergeCourseSyncPR(ctx, 33, 5)
	assert.NoError(t, err)
}

func TestMergeCourseSyncPR_NotFound(t *testing.T) {
	s, mr, _, _, _ := newCourseSyncService()
	ctx := context.Background()
	mr.On("GetCourseSyncPR", ctx, int64(33)).Return(nil, nil)

	err := s.MergeCourseSyncPR(ctx, 33, 5)
	assert.ErrorIs(t, err, ErrCourseSyncPRNotFound)
}

func TestMergeCourseSyncPR_NotPending(t *testing.T) {
	s, mr, _, _, _ := newCourseSyncService()
	ctx := context.Background()
	mr.On("GetCourseSyncPR", ctx, int64(33)).Return(&CourseSyncPR{
		ID: 33, Status: SyncPRStatusMerged, PullRequestID: 100,
	}, nil)

	err := s.MergeCourseSyncPR(ctx, 33, 5)
	assert.ErrorIs(t, err, ErrCourseSyncPRNotPending)
}

func TestMergeCourseSyncPR_ConflictFlipsStatus(t *testing.T) {
	s, mr, _, mu, mp := newCourseSyncService()
	ctx := context.Background()
	doer := &user_model.User{ID: 99, Name: "eduadmin"}
	conflictErr := errors.New("merge conflict")

	mr.On("GetCourseSyncPR", ctx, int64(33)).Return(&CourseSyncPR{
		ID: 33, Status: SyncPRStatusPending, PullRequestID: 100,
	}, nil)
	mu.On("GetUserByName", ctx, "eduadmin").Return(doer, nil)
	mp.On("MergePullRequest", ctx, mock.Anything).Return(conflictErr)
	mp.On("IsMergeConflictError", conflictErr).Return(true)
	mr.On("UpdateCourseSyncPR", ctx, mock.MatchedBy(func(p *CourseSyncPR) bool {
		return p.Status == SyncPRStatusConflict
	})).Return(nil)

	err := s.MergeCourseSyncPR(ctx, 33, 5)
	assert.Error(t, err)
}
