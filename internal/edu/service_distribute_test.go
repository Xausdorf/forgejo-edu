package edu

import (
	"context"
	"errors"
	"testing"

	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newDistributeService() (*service, *MockRepository, *MockRepoForker, *MockUserCreator) {
	mr := new(MockRepository)
	mf := new(MockRepoForker)
	mu := new(MockUserCreator)
	s := &service{repo: mr, forker: mf, users: mu}
	return s, mr, mf, mu
}

// ---------------------- CreateAssignment validation ----------------------

func TestCreateAssignment_RejectsEmptyTaskName(t *testing.T) {
	s, _, _, _ := newDistributeService()
	_, err := s.CreateAssignment(context.Background(), CreateAssignmentOptions{
		CourseID: 1, TaskName: "", Title: "T", AllowedFilesGlob: "tasks/x/x.cpp",
	})
	assert.Error(t, err)
}

func TestCreateAssignment_RejectsBadTaskName(t *testing.T) {
	s, _, _, _ := newDistributeService()
	_, err := s.CreateAssignment(context.Background(), CreateAssignmentOptions{
		CourseID: 1, TaskName: "Multiplication!", Title: "T", AllowedFilesGlob: "tasks/x/x.cpp",
	})
	assert.ErrorIs(t, err, ErrAssignmentTaskNameInvalid)
}

func TestCreateAssignment_RejectsTooLongTaskName(t *testing.T) {
	s, _, _, _ := newDistributeService()
	long := make([]byte, 101)
	for i := range long {
		long[i] = 'a'
	}
	_, err := s.CreateAssignment(context.Background(), CreateAssignmentOptions{
		CourseID: 1, TaskName: string(long), Title: "T", AllowedFilesGlob: "tasks/x/x.cpp",
	})
	assert.ErrorIs(t, err, ErrAssignmentTaskNameInvalid)
}

func TestCreateAssignment_RejectsMissingGlob(t *testing.T) {
	s, _, _, _ := newDistributeService()
	_, err := s.CreateAssignment(context.Background(), CreateAssignmentOptions{
		CourseID: 1, TaskName: "mul", Title: "T", AllowedFilesGlob: "",
	})
	assert.ErrorIs(t, err, ErrAllowedFilesGlobRequired)
}

func TestCreateAssignment_RejectsCourseWithoutTasksMaster(t *testing.T) {
	s, mr, _, _ := newDistributeService()
	ctx := context.Background()

	mr.On("GetCourseByID", ctx, int64(1)).Return(&Course{ID: 1, TasksMasterRepoID: 0}, nil)

	_, err := s.CreateAssignment(ctx, CreateAssignmentOptions{
		CourseID: 1, TaskName: "mul", Title: "T", AllowedFilesGlob: "tasks/mul/mul.cpp",
	})
	assert.ErrorIs(t, err, ErrTasksMasterRepoNotSet)
}

func TestCreateAssignment_RejectsDuplicateTaskName(t *testing.T) {
	s, mr, _, _ := newDistributeService()
	ctx := context.Background()

	mr.On("GetCourseByID", ctx, int64(1)).Return(&Course{ID: 1, TasksMasterRepoID: 99}, nil)
	mr.On("GetAssignmentByCourseAndTask", ctx, int64(1), "mul").Return(&Assignment{ID: 7, TaskName: "mul"}, nil)

	_, err := s.CreateAssignment(ctx, CreateAssignmentOptions{
		CourseID: 1, TaskName: "mul", Title: "T", AllowedFilesGlob: "tasks/mul/mul.cpp",
	})
	assert.ErrorIs(t, err, ErrAssignmentTaskNameInUse)
}

func TestCreateAssignment_RejectsMissingBranch(t *testing.T) {
	s, mr, mf, _ := newDistributeService()
	ctx := context.Background()

	mr.On("GetCourseByID", ctx, int64(1)).Return(&Course{ID: 1, TasksMasterRepoID: 99}, nil)
	mr.On("GetAssignmentByCourseAndTask", ctx, int64(1), "mul").Return(nil, nil)
	mf.On("BranchExists", ctx, int64(99), "submits/mul").Return(false, nil)

	_, err := s.CreateAssignment(ctx, CreateAssignmentOptions{
		CourseID: 1, TaskName: "mul", Title: "T", AllowedFilesGlob: "tasks/mul/mul.cpp",
	})
	assert.ErrorIs(t, err, ErrSubmitsBranchNotFound)
}

func TestCreateAssignment_HappyPath(t *testing.T) {
	s, mr, mf, _ := newDistributeService()
	ctx := context.Background()

	mr.On("GetCourseByID", ctx, int64(1)).Return(&Course{ID: 1, TasksMasterRepoID: 99}, nil)
	mr.On("GetAssignmentByCourseAndTask", ctx, int64(1), "mul").Return(nil, nil)
	mf.On("BranchExists", ctx, int64(99), "submits/mul").Return(true, nil)
	mr.On("CreateAssignment", ctx, mock.MatchedBy(func(a *Assignment) bool {
		return a.CourseID == 1 && a.TaskName == "mul" && a.AllowedFilesGlob == "tasks/mul/mul.cpp" && a.Title == "T"
	})).Return(nil)

	a, err := s.CreateAssignment(ctx, CreateAssignmentOptions{
		CourseID: 1, TaskName: "mul", Title: "T", AllowedFilesGlob: "tasks/mul/mul.cpp",
	})
	assert.NoError(t, err)
	assert.NotNil(t, a)
	assert.Equal(t, "mul", a.TaskName)
	mr.AssertExpectations(t)
	mf.AssertExpectations(t)
}

// ---------------------- DistributeAssignment validation ----------------------

func TestDistributeAssignment_RequiresUserCreator(t *testing.T) {
	mr := new(MockRepository)
	mf := new(MockRepoForker)
	s := &service{repo: mr, forker: mf, users: nil}

	_, err := s.DistributeAssignment(context.Background(), 1, 99)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user creator not configured")
}

func TestDistributeAssignment_RequiresAssignment(t *testing.T) {
	s, mr, _, _ := newDistributeService()
	ctx := context.Background()

	mr.On("GetAssignmentByID", ctx, int64(7)).Return(nil, nil)

	_, err := s.DistributeAssignment(ctx, 7, 99)
	assert.Error(t, err)
}

func TestDistributeAssignment_RequiresTasksMaster(t *testing.T) {
	s, mr, _, _ := newDistributeService()
	ctx := context.Background()

	mr.On("GetAssignmentByID", ctx, int64(7)).Return(&Assignment{ID: 7, CourseID: 1, TaskName: "mul"}, nil)
	mr.On("GetCourseByID", ctx, int64(1)).Return(&Course{ID: 1, TasksMasterRepoID: 0}, nil)

	_, err := s.DistributeAssignment(ctx, 7, 99)
	assert.ErrorIs(t, err, ErrTasksMasterRepoNotSet)
}

func TestDistributeAssignment_RequiresInitForksDone(t *testing.T) {
	s, mr, _, _ := newDistributeService()
	ctx := context.Background()

	mr.On("GetAssignmentByID", ctx, int64(7)).Return(&Assignment{ID: 7, CourseID: 1, TaskName: "mul"}, nil)
	mr.On("GetCourseByID", ctx, int64(1)).Return(&Course{ID: 1, TasksMasterRepoID: 99}, nil)
	mr.On("GetInitForksTaskByCourse", ctx, int64(1)).Return(&InitForksTask{ID: 5, Status: StatusRunning}, nil)

	_, err := s.DistributeAssignment(ctx, 7, 99)
	assert.ErrorIs(t, err, ErrInitForksNotDone)
}

func TestDistributeAssignment_NoStudents_MarksDone(t *testing.T) {
	s, mr, _, _ := newDistributeService()
	ctx := context.Background()

	mr.On("GetAssignmentByID", ctx, int64(7)).Return(&Assignment{ID: 7, CourseID: 1, TaskName: "mul"}, nil)
	mr.On("GetCourseByID", ctx, int64(1)).Return(&Course{ID: 1, TasksMasterRepoID: 99}, nil)
	mr.On("GetInitForksTaskByCourse", ctx, int64(1)).Return(&InitForksTask{ID: 5, Status: StatusDone}, nil)
	mr.On("GetEnrollments", ctx, int64(1)).Return([]*CourseEnrollment{
		{ID: 1, CourseID: 1, UserID: 100, Role: RoleTeacher},
	}, nil)
	mr.On("CreateDistributeTask", ctx, mock.MatchedBy(func(t *DistributeTask) bool {
		return t.AssignmentID == 7 && t.TotalEnrollments == 0
	})).Return(nil)
	mr.On("UpdateDistributeTask", ctx, mock.MatchedBy(func(t *DistributeTask) bool {
		return t.Status == StatusDone
	})).Return(nil)

	task, err := s.DistributeAssignment(ctx, 7, 99)
	assert.NoError(t, err)
	assert.Equal(t, StatusDone, task.Status)
	mr.AssertExpectations(t)
}

func TestDistributeAssignment_SkipsStudentsWithoutFork(t *testing.T) {
	s, mr, _, _ := newDistributeService()
	ctx := context.Background()

	mr.On("GetAssignmentByID", ctx, int64(7)).Return(&Assignment{ID: 7, CourseID: 1, TaskName: "mul"}, nil)
	mr.On("GetCourseByID", ctx, int64(1)).Return(&Course{ID: 1, TasksMasterRepoID: 99}, nil)
	mr.On("GetInitForksTaskByCourse", ctx, int64(1)).Return(&InitForksTask{ID: 5, Status: StatusDone}, nil)
	mr.On("GetEnrollments", ctx, int64(1)).Return([]*CourseEnrollment{
		{ID: 1, CourseID: 1, UserID: 42, Role: RoleStudent, StudentForkRepoID: 0},
	}, nil)
	mr.On("CreateDistributeTask", ctx, mock.MatchedBy(func(t *DistributeTask) bool {
		return t.TotalEnrollments == 0
	})).Return(nil)
	mr.On("UpdateDistributeTask", ctx, mock.MatchedBy(func(t *DistributeTask) bool {
		return t.Status == StatusDone
	})).Return(nil)

	task, err := s.DistributeAssignment(ctx, 7, 99)
	assert.NoError(t, err)
	assert.Equal(t, StatusDone, task.Status)
}

// ---------------------- distributeOne ----------------------

func TestDistributeOne_Idempotent_WhenSubmissionExists(t *testing.T) {
	s, mr, mf, _ := newDistributeService()
	ctx := context.Background()

	enr := &CourseEnrollment{ID: 7, UserID: 42, StudentForkRepoID: 555}
	assignment := &Assignment{ID: 1, TaskName: "mul"}
	doer := &user_model.User{ID: 1, Name: "eduadmin"}

	mr.On("GetSubmissionByEnrollmentAssignment", ctx, int64(7), int64(1)).Return(&Submission{ID: 99}, nil)

	err := s.distributeOne(ctx, &Course{}, assignment, "submits/mul", doer, enr)
	assert.NoError(t, err)
	mf.AssertNotCalled(t, "SyncFork", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	mr.AssertNotCalled(t, "CreateSubmission", mock.Anything, mock.Anything)
}

func TestDistributeOne_HappyPath_PushAndCreate(t *testing.T) {
	s, mr, mf, _ := newDistributeService()
	ctx := context.Background()

	enr := &CourseEnrollment{ID: 7, UserID: 42, StudentForkRepoID: 555}
	assignment := &Assignment{ID: 1, TaskName: "mul"}
	doer := &user_model.User{ID: 1, Name: "eduadmin"}
	forkRepo := &repo_model.Repository{ID: 555, Name: "alice-tasks"}

	mr.On("GetSubmissionByEnrollmentAssignment", ctx, int64(7), int64(1)).Return(nil, nil)
	mf.On("GetRepositoryByID", ctx, int64(555)).Return(forkRepo, nil)
	mf.On("SyncFork", ctx, doer, forkRepo, "submits/mul").Return(nil)
	mr.On("CreateSubmission", ctx, mock.MatchedBy(func(sub *Submission) bool {
		return sub.AssignmentID == 1 && sub.EnrollmentID == 7 && sub.UserID == 42 &&
			sub.BranchName == "submits/mul" && sub.Status == StatusSubmissionPending && sub.Grade == -1
	})).Return(nil)

	err := s.distributeOne(ctx, &Course{}, assignment, "submits/mul", doer, enr)
	assert.NoError(t, err)
	mr.AssertExpectations(t)
	mf.AssertExpectations(t)
}

func TestDistributeOne_PropagatesPushError(t *testing.T) {
	s, mr, mf, _ := newDistributeService()
	ctx := context.Background()

	enr := &CourseEnrollment{ID: 7, UserID: 42, StudentForkRepoID: 555}
	assignment := &Assignment{ID: 1, TaskName: "mul"}
	doer := &user_model.User{ID: 1, Name: "eduadmin"}
	forkRepo := &repo_model.Repository{ID: 555, Name: "alice-tasks"}

	mr.On("GetSubmissionByEnrollmentAssignment", ctx, int64(7), int64(1)).Return(nil, nil)
	mf.On("GetRepositoryByID", ctx, int64(555)).Return(forkRepo, nil)
	mf.On("SyncFork", ctx, doer, forkRepo, "submits/mul").Return(errors.New("boom"))

	err := s.distributeOne(ctx, &Course{}, assignment, "submits/mul", doer, enr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
	mr.AssertNotCalled(t, "CreateSubmission", mock.Anything, mock.Anything)
}
