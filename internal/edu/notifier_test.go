package edu

import (
	"context"
	"testing"

	actions_model "forgejo.org/models/actions"
	issues_model "forgejo.org/models/issues"
	user_model "forgejo.org/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newTestNotifier() (*EduNotifier, *MockRepository, *MockPullRequestService, *MockActionLogReader, *MockUserCreator) {
	repo := &MockRepository{}
	pulls := &MockPullRequestService{}
	logs := &MockActionLogReader{}
	users := &MockUserCreator{}
	n := &EduNotifier{
		repo:  repo,
		pulls: pulls,
		logs:  logs,
		users: users,
	}
	return n, repo, pulls, logs, users
}

func runFixture(repoID int64, status actions_model.Status) *actions_model.ActionRun {
	return &actions_model.ActionRun{
		ID:        1000,
		RepoID:    repoID,
		Ref:       "refs/heads/submits/multiplication",
		CommitSHA: "deadbeef",
		Status:    status,
	}
}

func TestProcessCompletedRun_NoEnrollment_Skips(t *testing.T) {
	n, repo, pulls, logs, users := newTestNotifier()
	repo.On("GetEnrollmentByForkRepo", mock.Anything, int64(42)).Return(nil, nil)

	err := n.processCompletedRun(context.Background(), runFixture(42, actions_model.StatusSuccess), "submits/multiplication")
	assert.NoError(t, err)

	repo.AssertExpectations(t)
	pulls.AssertNotCalled(t, "CreatePullRequest", mock.Anything, mock.Anything)
	logs.AssertNotCalled(t, "ReadRunLogLines", mock.Anything, mock.Anything)
	users.AssertNotCalled(t, "GetUserByName", mock.Anything, mock.Anything)
}

func TestProcessCompletedRun_NoAssignment_Skips(t *testing.T) {
	n, repo, pulls, _, users := newTestNotifier()
	enr := &CourseEnrollment{ID: 5, CourseID: 1, UserID: 7, StudentForkRepoID: 42}
	repo.On("GetEnrollmentByForkRepo", mock.Anything, int64(42)).Return(enr, nil)
	repo.On("GetAssignmentByCourseAndTask", mock.Anything, int64(1), "multiplication").Return(nil, nil)

	err := n.processCompletedRun(context.Background(), runFixture(42, actions_model.StatusSuccess), "submits/multiplication")
	assert.NoError(t, err)

	users.AssertNotCalled(t, "GetUserByName", mock.Anything, mock.Anything)
	pulls.AssertNotCalled(t, "CreatePullRequest", mock.Anything, mock.Anything)
}

func TestProcessCompletedRun_NoSubmission_Skips(t *testing.T) {
	n, repo, _, _, _ := newTestNotifier()
	enr := &CourseEnrollment{ID: 5, CourseID: 1, UserID: 7, StudentForkRepoID: 42}
	asn := &Assignment{ID: 11, CourseID: 1, TaskName: "multiplication", AllowedFilesGlob: "tasks/multiplication/*.cpp"}
	repo.On("GetEnrollmentByForkRepo", mock.Anything, int64(42)).Return(enr, nil)
	repo.On("GetAssignmentByCourseAndTask", mock.Anything, int64(1), "multiplication").Return(asn, nil)
	repo.On("GetSubmissionByEnrollmentAssignment", mock.Anything, int64(5), int64(11)).Return(nil, nil)

	err := n.processCompletedRun(context.Background(), runFixture(42, actions_model.StatusSuccess), "submits/multiplication")
	assert.NoError(t, err)
}

func TestProcessCompletedRun_ConstraintViolation_CommentsAndFails(t *testing.T) {
	n, repo, pulls, logs, users := newTestNotifier()
	enr := &CourseEnrollment{ID: 5, CourseID: 1, UserID: 7, StudentForkRepoID: 42}
	asn := &Assignment{ID: 11, CourseID: 1, TaskName: "multiplication", AllowedFilesGlob: "tasks/multiplication/*.cpp"}
	sub := &Submission{ID: 99, AssignmentID: 11, EnrollmentID: 5, UserID: 7, BranchName: "submits/multiplication", Status: StatusSubmissionPending}
	course := &Course{ID: 1, Name: "C++ 2026"}
	doer := &user_model.User{ID: 1, Name: "eduadmin"}

	repo.On("GetEnrollmentByForkRepo", mock.Anything, int64(42)).Return(enr, nil)
	repo.On("GetAssignmentByCourseAndTask", mock.Anything, int64(1), "multiplication").Return(asn, nil)
	repo.On("GetSubmissionByEnrollmentAssignment", mock.Anything, int64(5), int64(11)).Return(sub, nil)
	users.On("GetUserByName", mock.Anything, "eduadmin").Return(doer, nil)
	repo.On("GetCourseByID", mock.Anything, int64(1)).Return(course, nil)

	pulls.On("CreatePullRequest", mock.Anything, mock.MatchedBy(func(opts CreatePullRequestOptions) bool {
		return opts.BaseRepoID == 42 && opts.HeadRepoID == 42 && opts.HeadBranch == "submits/multiplication" && opts.BaseBranch == "main"
	})).Return(&issues_model.PullRequest{ID: 555}, nil)
	repo.On("UpdateSubmissionPullRequestID", mock.Anything, int64(99), int64(555)).Return(nil)

	pulls.On("GetBranchChangedFiles", mock.Anything, int64(42), "submits/multiplication", "main").
		Return([]string{"tasks/multiplication/a.cpp", "README.md"}, nil)

	pulls.On("AddPullRequestComment", mock.Anything, int64(555), mock.MatchedBy(func(body string) bool {
		return assert.Contains(t, body, "README.md") && assert.Contains(t, body, "Constraint check failed")
	}), doer).Return(&issues_model.Comment{ID: 1}, nil)

	repo.On("CreateTestResult", mock.Anything, mock.MatchedBy(func(tr *TestResult) bool {
		return tr.SubmissionID == 99 && tr.Score == 0 && tr.CommitSHA == "deadbeef"
	})).Return(nil)
	repo.On("UpdateSubmission", mock.Anything, mock.MatchedBy(func(s *Submission) bool {
		return s.ID == 99 && s.Status == StatusSubmissionFailed
	})).Return(nil)

	err := n.processCompletedRun(context.Background(), runFixture(42, actions_model.StatusSuccess), "submits/multiplication")
	assert.NoError(t, err)

	logs.AssertNotCalled(t, "ReadRunLogLines", mock.Anything, mock.Anything)
	repo.AssertExpectations(t)
	pulls.AssertExpectations(t)
}

func TestProcessCompletedRun_Success_AutoGrades(t *testing.T) {
	n, repo, pulls, logs, users := newTestNotifier()
	enr := &CourseEnrollment{ID: 5, CourseID: 1, UserID: 7, StudentForkRepoID: 42}
	asn := &Assignment{ID: 11, CourseID: 1, TaskName: "multiplication", AllowedFilesGlob: "tasks/multiplication/*.cpp"}
	sub := &Submission{ID: 99, AssignmentID: 11, EnrollmentID: 5, UserID: 7, PullRequestID: 555, BranchName: "submits/multiplication", Status: StatusSubmissionPending, ManualGrade: false}
	doer := &user_model.User{ID: 1, Name: "eduadmin"}
	course := &Course{ID: 1, Name: "C++ 2026"}

	repo.On("GetEnrollmentByForkRepo", mock.Anything, int64(42)).Return(enr, nil)
	repo.On("GetAssignmentByCourseAndTask", mock.Anything, int64(1), "multiplication").Return(asn, nil)
	repo.On("GetSubmissionByEnrollmentAssignment", mock.Anything, int64(5), int64(11)).Return(sub, nil)
	users.On("GetUserByName", mock.Anything, "eduadmin").Return(doer, nil)
	repo.On("GetCourseByID", mock.Anything, int64(1)).Return(course, nil)
	pulls.On("GetBranchChangedFiles", mock.Anything, int64(42), "submits/multiplication", "main").
		Return([]string{"tasks/multiplication/a.cpp"}, nil)
	logs.On("ReadRunLogLines", mock.Anything, int64(1000)).Return([]string{"some output", "::edu-grade::95"}, nil)
	repo.On("CreateTestResult", mock.Anything, mock.MatchedBy(func(tr *TestResult) bool {
		return tr.SubmissionID == 99 && tr.Score == 95
	})).Return(nil)
	repo.On("AutoGradeSubmission", mock.Anything, int64(99), 95).Return(nil)

	err := n.processCompletedRun(context.Background(), runFixture(42, actions_model.StatusSuccess), "submits/multiplication")
	assert.NoError(t, err)

	pulls.AssertNotCalled(t, "CreatePullRequest", mock.Anything, mock.Anything)
	pulls.AssertNotCalled(t, "AddPullRequestComment", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "UpdateSubmissionPullRequestID", mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "UpdateSubmission", mock.Anything, mock.Anything)
}

func TestProcessCompletedRun_Success_ManualGradePreserved(t *testing.T) {
	n, repo, pulls, logs, users := newTestNotifier()
	enr := &CourseEnrollment{ID: 5, CourseID: 1, UserID: 7, StudentForkRepoID: 42}
	asn := &Assignment{ID: 11, CourseID: 1, TaskName: "multiplication", AllowedFilesGlob: "tasks/multiplication/*.cpp"}
	sub := &Submission{ID: 99, AssignmentID: 11, EnrollmentID: 5, UserID: 7, PullRequestID: 555, ManualGrade: true, Grade: 70}
	doer := &user_model.User{ID: 1, Name: "eduadmin"}
	course := &Course{ID: 1, Name: "C++ 2026"}

	repo.On("GetEnrollmentByForkRepo", mock.Anything, int64(42)).Return(enr, nil)
	repo.On("GetAssignmentByCourseAndTask", mock.Anything, int64(1), "multiplication").Return(asn, nil)
	repo.On("GetSubmissionByEnrollmentAssignment", mock.Anything, int64(5), int64(11)).Return(sub, nil)
	users.On("GetUserByName", mock.Anything, "eduadmin").Return(doer, nil)
	repo.On("GetCourseByID", mock.Anything, int64(1)).Return(course, nil)
	pulls.On("GetBranchChangedFiles", mock.Anything, int64(42), "submits/multiplication", "main").
		Return([]string{"tasks/multiplication/a.cpp"}, nil)
	logs.On("ReadRunLogLines", mock.Anything, int64(1000)).Return([]string{"::edu-grade::95"}, nil)
	repo.On("CreateTestResult", mock.Anything, mock.Anything).Return(nil)
	repo.On("UpdateSubmission", mock.Anything, mock.MatchedBy(func(s *Submission) bool {
		return s.ID == 99 && s.Status == StatusSubmissionDone
	})).Return(nil)

	err := n.processCompletedRun(context.Background(), runFixture(42, actions_model.StatusSuccess), "submits/multiplication")
	assert.NoError(t, err)

	repo.AssertNotCalled(t, "AutoGradeSubmission", mock.Anything, mock.Anything, mock.Anything)
}

func TestProcessCompletedRun_CIFailure_NoAutoGrade(t *testing.T) {
	n, repo, pulls, logs, users := newTestNotifier()
	enr := &CourseEnrollment{ID: 5, CourseID: 1, UserID: 7, StudentForkRepoID: 42}
	asn := &Assignment{ID: 11, CourseID: 1, TaskName: "multiplication", AllowedFilesGlob: "tasks/multiplication/*.cpp"}
	sub := &Submission{ID: 99, AssignmentID: 11, EnrollmentID: 5, UserID: 7, PullRequestID: 555}
	doer := &user_model.User{ID: 1, Name: "eduadmin"}
	course := &Course{ID: 1, Name: "C++ 2026"}

	repo.On("GetEnrollmentByForkRepo", mock.Anything, int64(42)).Return(enr, nil)
	repo.On("GetAssignmentByCourseAndTask", mock.Anything, int64(1), "multiplication").Return(asn, nil)
	repo.On("GetSubmissionByEnrollmentAssignment", mock.Anything, int64(5), int64(11)).Return(sub, nil)
	users.On("GetUserByName", mock.Anything, "eduadmin").Return(doer, nil)
	repo.On("GetCourseByID", mock.Anything, int64(1)).Return(course, nil)
	pulls.On("GetBranchChangedFiles", mock.Anything, int64(42), "submits/multiplication", "main").
		Return([]string{"tasks/multiplication/a.cpp"}, nil)
	logs.On("ReadRunLogLines", mock.Anything, int64(1000)).Return([]string{"build failed"}, nil)
	repo.On("CreateTestResult", mock.Anything, mock.MatchedBy(func(tr *TestResult) bool {
		return tr.Score == 0
	})).Return(nil)
	repo.On("UpdateSubmission", mock.Anything, mock.MatchedBy(func(s *Submission) bool {
		return s.Status == StatusSubmissionFailed
	})).Return(nil)

	err := n.processCompletedRun(context.Background(), runFixture(42, actions_model.StatusFailure), "submits/multiplication")
	assert.NoError(t, err)

	repo.AssertNotCalled(t, "AutoGradeSubmission", mock.Anything, mock.Anything, mock.Anything)
}

func TestActionRunNowDone_NonSubmitsBranch_Skips(t *testing.T) {
	n, repo, _, _, _ := newTestNotifier()
	run := &actions_model.ActionRun{ID: 1, RepoID: 42, Ref: "refs/heads/main", Status: actions_model.StatusSuccess}
	n.ActionRunNowDone(context.Background(), run, actions_model.StatusRunning, nil)
	repo.AssertNotCalled(t, "GetEnrollmentByForkRepo", mock.Anything, mock.Anything)
}

func TestActionRunNowDone_TagRef_Skips(t *testing.T) {
	n, repo, _, _, _ := newTestNotifier()
	run := &actions_model.ActionRun{ID: 1, RepoID: 42, Ref: "refs/tags/v1.0", Status: actions_model.StatusSuccess}
	n.ActionRunNowDone(context.Background(), run, actions_model.StatusRunning, nil)
	repo.AssertNotCalled(t, "GetEnrollmentByForkRepo", mock.Anything, mock.Anything)
}

func TestProcessCompletedRun_PrefixesPRTitleWithGroup(t *testing.T) {
	n, repo, pulls, logs, users := newTestNotifier()
	enr := &CourseEnrollment{ID: 5, CourseID: 1, UserID: 7, StudentForkRepoID: 42, GroupName: "SE-241"}
	asn := &Assignment{ID: 11, CourseID: 1, TaskName: "multiplication", AllowedFilesGlob: "tasks/multiplication/*.cpp"}
	sub := &Submission{ID: 99, AssignmentID: 11, EnrollmentID: 5, UserID: 7, BranchName: "submits/multiplication", Status: StatusSubmissionPending}
	course := &Course{ID: 1, Name: "C++ 2026"}
	doer := &user_model.User{ID: 1, Name: "eduadmin"}

	repo.On("GetEnrollmentByForkRepo", mock.Anything, int64(42)).Return(enr, nil)
	repo.On("GetAssignmentByCourseAndTask", mock.Anything, int64(1), "multiplication").Return(asn, nil)
	repo.On("GetSubmissionByEnrollmentAssignment", mock.Anything, int64(5), int64(11)).Return(sub, nil)
	users.On("GetUserByName", mock.Anything, "eduadmin").Return(doer, nil)
	repo.On("GetCourseByID", mock.Anything, int64(1)).Return(course, nil)

	pulls.On("CreatePullRequest", mock.Anything, mock.MatchedBy(func(opts CreatePullRequestOptions) bool {
		return opts.Title == "[SE-241] Submit: multiplication"
	})).Return(&issues_model.PullRequest{ID: 555}, nil)
	repo.On("UpdateSubmissionPullRequestID", mock.Anything, int64(99), int64(555)).Return(nil)

	pulls.On("GetBranchChangedFiles", mock.Anything, int64(42), "submits/multiplication", "main").
		Return([]string{"tasks/multiplication/a.cpp"}, nil)

	logs.On("ReadRunLogLines", mock.Anything, int64(1000)).Return([]string{"::edu-grade::95"}, nil)
	repo.On("CreateTestResult", mock.Anything, mock.AnythingOfType("*edu.TestResult")).Return(nil)
	repo.On("AutoGradeSubmission", mock.Anything, int64(99), 95).Return(nil)

	err := n.processCompletedRun(context.Background(), runFixture(42, actions_model.StatusSuccess), "submits/multiplication")
	assert.NoError(t, err)

	repo.AssertExpectations(t)
	pulls.AssertExpectations(t)
}

func TestProcessCompletedRun_NoGroup_NoPrefix(t *testing.T) {
	n, repo, pulls, logs, users := newTestNotifier()
	enr := &CourseEnrollment{ID: 5, CourseID: 1, UserID: 7, StudentForkRepoID: 42}
	asn := &Assignment{ID: 11, CourseID: 1, TaskName: "multiplication", AllowedFilesGlob: "tasks/multiplication/*.cpp"}
	sub := &Submission{ID: 99, AssignmentID: 11, EnrollmentID: 5, UserID: 7, BranchName: "submits/multiplication", Status: StatusSubmissionPending}
	course := &Course{ID: 1, Name: "C++ 2026"}
	doer := &user_model.User{ID: 1, Name: "eduadmin"}

	repo.On("GetEnrollmentByForkRepo", mock.Anything, int64(42)).Return(enr, nil)
	repo.On("GetAssignmentByCourseAndTask", mock.Anything, int64(1), "multiplication").Return(asn, nil)
	repo.On("GetSubmissionByEnrollmentAssignment", mock.Anything, int64(5), int64(11)).Return(sub, nil)
	users.On("GetUserByName", mock.Anything, "eduadmin").Return(doer, nil)
	repo.On("GetCourseByID", mock.Anything, int64(1)).Return(course, nil)

	pulls.On("CreatePullRequest", mock.Anything, mock.MatchedBy(func(opts CreatePullRequestOptions) bool {
		return opts.Title == "Submit: multiplication"
	})).Return(&issues_model.PullRequest{ID: 555}, nil)
	repo.On("UpdateSubmissionPullRequestID", mock.Anything, int64(99), int64(555)).Return(nil)

	pulls.On("GetBranchChangedFiles", mock.Anything, int64(42), "submits/multiplication", "main").
		Return([]string{"tasks/multiplication/a.cpp"}, nil)

	logs.On("ReadRunLogLines", mock.Anything, int64(1000)).Return([]string{"::edu-grade::100"}, nil)
	repo.On("CreateTestResult", mock.Anything, mock.AnythingOfType("*edu.TestResult")).Return(nil)
	repo.On("AutoGradeSubmission", mock.Anything, int64(99), 100).Return(nil)

	err := n.processCompletedRun(context.Background(), runFixture(42, actions_model.StatusSuccess), "submits/multiplication")
	assert.NoError(t, err)

	repo.AssertExpectations(t)
	pulls.AssertExpectations(t)
}
