package edu

import (
	"context"
	"errors"
	"testing"

	"forgejo.org/models/perm"
	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newInitForksService() (*service, *MockRepository, *MockRepoForker, *MockUserCreator) {
	mr := new(MockRepository)
	mf := new(MockRepoForker)
	mu := new(MockUserCreator)
	s := &service{repo: mr, forker: mf, users: mu}
	return s, mr, mf, mu
}

func TestInitCourseForks_RequiresTasksMasterRepo(t *testing.T) {
	s, mr, _, _ := newInitForksService()
	ctx := context.Background()

	mr.On("GetCourseByID", ctx, int64(10)).Return(&Course{ID: 10, OrgID: 5, TasksMasterRepoID: 0}, nil)

	_, err := s.InitCourseForks(ctx, 10, 1)
	assert.ErrorIs(t, err, ErrTasksMasterRepoNotSet)
}

func TestInitCourseForks_RequiresOrg(t *testing.T) {
	s, mr, _, _ := newInitForksService()
	ctx := context.Background()

	mr.On("GetCourseByID", ctx, int64(10)).Return(&Course{ID: 10, OrgID: 0, TasksMasterRepoID: 99}, nil)

	_, err := s.InitCourseForks(ctx, 10, 1)
	assert.ErrorIs(t, err, ErrCourseHasNoOrg)
}

func TestInitCourseForks_RequiresUserCreator(t *testing.T) {
	mr := new(MockRepository)
	mf := new(MockRepoForker)
	s := &service{repo: mr, forker: mf, users: nil}
	ctx := context.Background()

	_, err := s.InitCourseForks(ctx, 10, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user creator not configured")
}

func TestInitCourseForks_NoStudents_MarksDone(t *testing.T) {
	s, mr, mf, _ := newInitForksService()
	ctx := context.Background()

	mr.On("GetCourseByID", ctx, int64(10)).Return(&Course{ID: 10, OrgID: 5, TasksMasterRepoID: 99}, nil)
	mf.On("GetRepositoryByID", ctx, int64(99)).Return(&repo_model.Repository{ID: 99, Name: "tasks-master"}, nil)
	mr.On("GetEnrollments", ctx, int64(10)).Return([]*CourseEnrollment{
		{ID: 1, CourseID: 10, UserID: 1, Role: RoleTeacher},
	}, nil)
	mr.On("CreateInitForksTask", ctx, mock.MatchedBy(func(t *InitForksTask) bool {
		return t.CourseID == 10 && t.TotalUsers == 0
	})).Return(nil)
	mr.On("UpdateInitForksTask", ctx, mock.MatchedBy(func(t *InitForksTask) bool {
		return t.Status == StatusDone
	})).Return(nil)

	task, err := s.InitCourseForks(ctx, 10, 1)
	assert.NoError(t, err)
	assert.Equal(t, StatusDone, task.Status)
	mr.AssertExpectations(t)
}

func TestInitOneFork_CreatesFork_WhenNotPresent(t *testing.T) {
	s, mr, mf, mu := newInitForksService()
	ctx := context.Background()

	course := &Course{ID: 10, OrgID: 5, TasksMasterRepoID: 99}
	baseRepo := &repo_model.Repository{ID: 99, Name: "tasks-master"}
	doer := &user_model.User{ID: 1, Name: "eduadmin"}
	org := &user_model.User{ID: 5, Name: "programming-cxx"}
	enr := &CourseEnrollment{ID: 7, CourseID: 10, UserID: 42, Role: RoleStudent}
	student := &user_model.User{ID: 42, Name: "alice"}
	createdFork := &repo_model.Repository{ID: 555, Name: "alice-tasks"}

	mu.On("GetUserByID", ctx, int64(42)).Return(student, nil)
	mf.On("GetRepositoryByOwnerAndName", ctx, int64(5), "alice-tasks").Return(nil, nil)
	mf.On("ForkRepositoryAndUpdates", ctx, doer, org, mock.MatchedBy(func(opts ForkRepoOptions) bool {
		return opts.Name == "alice-tasks" && opts.BaseRepo.ID == int64(99)
	})).Return(createdFork, nil)
	mr.On("UpdateEnrollment", ctx, mock.MatchedBy(func(e *CourseEnrollment) bool {
		return e.ID == int64(7) && e.StudentForkRepoID == int64(555)
	})).Return(nil)
	mf.On("EnableActionsUnit", ctx, int64(555)).Return(nil)
	mf.On("AddCollaborator", ctx, int64(555), int64(42), perm.AccessModeWrite).Return(nil)
	mf.On("GetDefaultBranch", ctx, int64(555)).Return("main", nil)
	mf.On("ProtectMainBranch", ctx, int64(555), "main").Return(nil)

	err := s.initOneFork(ctx, course, baseRepo, doer, org, enr)
	assert.NoError(t, err)
	assert.Equal(t, int64(555), enr.StudentForkRepoID)
	mr.AssertExpectations(t)
	mf.AssertExpectations(t)
}

func TestInitOneFork_ReusesExistingRepoInOrg(t *testing.T) {
	s, mr, mf, mu := newInitForksService()
	ctx := context.Background()

	course := &Course{ID: 10, OrgID: 5, TasksMasterRepoID: 99}
	baseRepo := &repo_model.Repository{ID: 99, Name: "tasks-master"}
	doer := &user_model.User{ID: 1, Name: "eduadmin"}
	org := &user_model.User{ID: 5, Name: "programming-cxx"}
	enr := &CourseEnrollment{ID: 7, CourseID: 10, UserID: 42, Role: RoleStudent}
	student := &user_model.User{ID: 42, Name: "alice"}
	existingRepo := &repo_model.Repository{ID: 777, Name: "alice-tasks"}

	mu.On("GetUserByID", ctx, int64(42)).Return(student, nil)
	mf.On("GetRepositoryByOwnerAndName", ctx, int64(5), "alice-tasks").Return(existingRepo, nil)
	mr.On("UpdateEnrollment", ctx, mock.MatchedBy(func(e *CourseEnrollment) bool {
		return e.StudentForkRepoID == int64(777)
	})).Return(nil)
	mf.On("EnableActionsUnit", ctx, int64(777)).Return(nil)
	mf.On("AddCollaborator", ctx, int64(777), int64(42), perm.AccessModeWrite).Return(nil)
	mf.On("GetDefaultBranch", ctx, int64(777)).Return("main", nil)
	mf.On("ProtectMainBranch", ctx, int64(777), "main").Return(nil)

	err := s.initOneFork(ctx, course, baseRepo, doer, org, enr)
	assert.NoError(t, err)
	mf.AssertNotCalled(t, "ForkRepositoryAndUpdates", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestInitOneFork_Idempotent_WhenForkAlreadyTracked(t *testing.T) {
	s, mr, mf, mu := newInitForksService()
	ctx := context.Background()

	course := &Course{ID: 10, OrgID: 5, TasksMasterRepoID: 99}
	baseRepo := &repo_model.Repository{ID: 99, Name: "tasks-master"}
	doer := &user_model.User{ID: 1, Name: "eduadmin"}
	org := &user_model.User{ID: 5, Name: "programming-cxx"}
	enr := &CourseEnrollment{ID: 7, CourseID: 10, UserID: 42, Role: RoleStudent, StudentForkRepoID: 555}
	student := &user_model.User{ID: 42, Name: "alice"}
	existingFork := &repo_model.Repository{ID: 555, Name: "alice-tasks"}

	mu.On("GetUserByID", ctx, int64(42)).Return(student, nil)
	mf.On("GetRepositoryByID", ctx, int64(555)).Return(existingFork, nil)
	mf.On("EnableActionsUnit", ctx, int64(555)).Return(nil)
	mf.On("AddCollaborator", ctx, int64(555), int64(42), perm.AccessModeWrite).Return(nil)
	mf.On("GetDefaultBranch", ctx, int64(555)).Return("main", nil)
	mf.On("ProtectMainBranch", ctx, int64(555), "main").Return(nil)

	err := s.initOneFork(ctx, course, baseRepo, doer, org, enr)
	assert.NoError(t, err)
	mf.AssertNotCalled(t, "ForkRepositoryAndUpdates", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	mr.AssertNotCalled(t, "UpdateEnrollment", mock.Anything, mock.Anything)
}

func TestInitOneFork_PropagatesForkError(t *testing.T) {
	s, _, mf, mu := newInitForksService()
	ctx := context.Background()

	course := &Course{ID: 10, OrgID: 5, TasksMasterRepoID: 99}
	baseRepo := &repo_model.Repository{ID: 99, Name: "tasks-master"}
	doer := &user_model.User{ID: 1, Name: "eduadmin"}
	org := &user_model.User{ID: 5, Name: "programming-cxx"}
	enr := &CourseEnrollment{ID: 7, CourseID: 10, UserID: 42, Role: RoleStudent}
	student := &user_model.User{ID: 42, Name: "alice"}

	mu.On("GetUserByID", ctx, int64(42)).Return(student, nil)
	mf.On("GetRepositoryByOwnerAndName", ctx, int64(5), "alice-tasks").Return(nil, nil)
	mf.On("ForkRepositoryAndUpdates", ctx, doer, org, mock.Anything).Return(nil, errors.New("boom"))

	err := s.initOneFork(ctx, course, baseRepo, doer, org, enr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "alice")
	assert.Contains(t, err.Error(), "boom")
}

func TestEnsureForkForEnrollment_NoOp_WhenNoTasksMaster(t *testing.T) {
	s, mr, _, _ := newInitForksService()
	ctx := context.Background()

	mr.On("GetCourseByID", ctx, int64(10)).Return(&Course{ID: 10, OrgID: 5, TasksMasterRepoID: 0}, nil)

	err := s.ensureForkForEnrollment(ctx, 10, 42)
	assert.NoError(t, err)
}

func TestEnsureForkForEnrollment_NoOp_WhenNoPriorInitDone(t *testing.T) {
	s, mr, _, _ := newInitForksService()
	ctx := context.Background()

	mr.On("GetCourseByID", ctx, int64(10)).Return(&Course{ID: 10, OrgID: 5, TasksMasterRepoID: 99}, nil)
	mr.On("GetInitForksTaskByCourse", ctx, int64(10)).Return(nil, nil)

	err := s.ensureForkForEnrollment(ctx, 10, 42)
	assert.NoError(t, err)
}

func TestEnsureForkForEnrollment_NoOp_WhenRoleIsNotStudent(t *testing.T) {
	s, mr, _, _ := newInitForksService()
	ctx := context.Background()

	mr.On("GetCourseByID", ctx, int64(10)).Return(&Course{ID: 10, OrgID: 5, TasksMasterRepoID: 99}, nil)
	mr.On("GetInitForksTaskByCourse", ctx, int64(10)).Return(&InitForksTask{ID: 1, Status: StatusDone}, nil)
	mr.On("GetEnrollmentByCourseUser", ctx, int64(10), int64(42)).Return(&CourseEnrollment{
		ID: 7, CourseID: 10, UserID: 42, Role: RoleTA,
	}, nil)

	err := s.ensureForkForEnrollment(ctx, 10, 42)
	assert.NoError(t, err)
}

func TestEnsureCollaboratorRemovedForEnrollment_NoOp_WhenNoFork(t *testing.T) {
	s, _, _, _ := newInitForksService()
	ctx := context.Background()

	err := s.ensureCollaboratorRemovedForEnrollment(ctx, 10, 42, 0)
	assert.NoError(t, err)
}

func TestEnsureCollaboratorRemovedForEnrollment_CallsAdapter(t *testing.T) {
	s, _, mf, _ := newInitForksService()
	ctx := context.Background()

	mf.On("RemoveCollaborator", ctx, int64(555), int64(42)).Return(nil)

	err := s.ensureCollaboratorRemovedForEnrollment(ctx, 10, 42, 555)
	assert.NoError(t, err)
	mf.AssertExpectations(t)
}
