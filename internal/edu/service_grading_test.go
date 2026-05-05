package edu

import (
	"context"
	"errors"
	"testing"

	issues_model "forgejo.org/models/issues"
	user_model "forgejo.org/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// newGradingTestService wires a service with mocked Repository, RepoForker,
// PullRequestService and UserCreator suitable for grading-flow unit tests.
// PullRequestService is wired via the variadic users[0] type-assert path
// (see NewService): we pass a combined mock that implements both
// UserCreator and PullRequestService.
type combinedMock struct {
	*MockUserCreator
	*MockPullRequestService
}

func newGradingTestService(t *testing.T) (*service, *MockRepository, *MockPullRequestService, *MockUserCreator) {
	t.Helper()
	repo := new(MockRepository)
	forker := new(MockRepoForker)
	users := new(MockUserCreator)
	pulls := new(MockPullRequestService)
	combined := combinedMock{users, pulls}
	svc := NewService(repo, forker, combined).(*service)
	return svc, repo, pulls, users
}

func TestApproveSubmission_Valid(t *testing.T) {
	svc, repo, _, _ := newGradingTestService(t)
	repo.On("ApproveSubmission", mock.Anything, int64(1), 85, "Good work", int64(50)).Return(nil)

	err := svc.ApproveSubmission(context.Background(), 1, 85, "Good work", 50)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestApproveSubmission_GradeOutOfRange(t *testing.T) {
	svc, _, _, _ := newGradingTestService(t)

	err := svc.ApproveSubmission(context.Background(), 1, 101, "", 50)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "between 0 and 100")

	err = svc.ApproveSubmission(context.Background(), 1, -1, "", 50)
	assert.Error(t, err)
}

func TestApproveSubmission_CommentTooLong(t *testing.T) {
	svc, _, _, _ := newGradingTestService(t)
	long := make([]byte, 10001)
	for i := range long {
		long[i] = 'x'
	}
	err := svc.ApproveSubmission(context.Background(), 1, 50, string(long), 50)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too long")
}

func TestMergeSubmission_HappyPath(t *testing.T) {
	svc, repo, pulls, users := newGradingTestService(t)
	sub := &Submission{ID: 7, Status: StatusSubmissionApproved, PullRequestID: 42}
	doer := &user_model.User{ID: 99, Name: "eduadmin"}

	repo.On("GetSubmissionByID", mock.Anything, int64(7)).Return(sub, nil)
	users.On("GetUserByName", mock.Anything, "eduadmin").Return(doer, nil)
	pulls.On("MergePullRequest", mock.Anything, mock.MatchedBy(func(opts MergePullRequestOptions) bool {
		return opts.PullRequestID == 42 && opts.Doer == doer && opts.MergeStyle == "merge"
	})).Return(nil)
	repo.On("MarkSubmissionMerged", mock.Anything, int64(7)).Return(nil)

	err := svc.MergeSubmission(context.Background(), 7)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
	pulls.AssertExpectations(t)
}

func TestMergeSubmission_NotApproved(t *testing.T) {
	svc, repo, _, _ := newGradingTestService(t)
	sub := &Submission{ID: 7, Status: StatusSubmissionDone, PullRequestID: 42}
	repo.On("GetSubmissionByID", mock.Anything, int64(7)).Return(sub, nil)

	err := svc.MergeSubmission(context.Background(), 7)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "approved before merging")
}

func TestMergeSubmission_NoPR(t *testing.T) {
	svc, repo, _, _ := newGradingTestService(t)
	sub := &Submission{ID: 7, Status: StatusSubmissionApproved, PullRequestID: 0}
	repo.On("GetSubmissionByID", mock.Anything, int64(7)).Return(sub, nil)

	err := svc.MergeSubmission(context.Background(), 7)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no pull request")
}

func TestMergeSubmission_DoerNotFound(t *testing.T) {
	svc, repo, _, users := newGradingTestService(t)
	sub := &Submission{ID: 7, Status: StatusSubmissionApproved, PullRequestID: 42}
	repo.On("GetSubmissionByID", mock.Anything, int64(7)).Return(sub, nil)
	users.On("GetUserByName", mock.Anything, "eduadmin").Return(nil, errors.New("not found"))

	err := svc.MergeSubmission(context.Background(), 7)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "eduadmin")
}

func TestMergeSubmission_MergeFails(t *testing.T) {
	svc, repo, pulls, users := newGradingTestService(t)
	sub := &Submission{ID: 7, Status: StatusSubmissionApproved, PullRequestID: 42}
	doer := &user_model.User{ID: 99, Name: "eduadmin"}
	repo.On("GetSubmissionByID", mock.Anything, int64(7)).Return(sub, nil)
	users.On("GetUserByName", mock.Anything, "eduadmin").Return(doer, nil)
	pulls.On("MergePullRequest", mock.Anything, mock.Anything).Return(errors.New("conflict"))

	err := svc.MergeSubmission(context.Background(), 7)
	assert.Error(t, err)
	repo.AssertNotCalled(t, "MarkSubmissionMerged", mock.Anything, mock.Anything)
}

func TestAddSubmissionComment_HappyPath(t *testing.T) {
	svc, repo, pulls, _ := newGradingTestService(t)
	doer := &user_model.User{ID: 11, Name: "ta1"}
	sub := &Submission{ID: 5, PullRequestID: 33}
	repo.On("GetSubmissionByID", mock.Anything, int64(5)).Return(sub, nil)
	pulls.On("AddPullRequestComment", mock.Anything, int64(33), "looks good", doer).
		Return(&issues_model.Comment{ID: 100}, nil)

	err := svc.AddSubmissionComment(context.Background(), 5, "looks good", doer)
	assert.NoError(t, err)
}

func TestAddSubmissionComment_EmptyBody(t *testing.T) {
	svc, _, _, _ := newGradingTestService(t)
	doer := &user_model.User{ID: 11}
	err := svc.AddSubmissionComment(context.Background(), 5, "   ", doer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestAddSubmissionComment_NoPR(t *testing.T) {
	svc, repo, _, _ := newGradingTestService(t)
	doer := &user_model.User{ID: 11}
	sub := &Submission{ID: 5, PullRequestID: 0}
	repo.On("GetSubmissionByID", mock.Anything, int64(5)).Return(sub, nil)

	err := svc.AddSubmissionComment(context.Background(), 5, "looks good", doer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no pull request")
}

func TestAddSubmissionComment_NilDoer(t *testing.T) {
	svc, _, _, _ := newGradingTestService(t)
	err := svc.AddSubmissionComment(context.Background(), 5, "x", nil)
	assert.Error(t, err)
}

func TestResetApproval_DelegatesToRepo(t *testing.T) {
	svc, repo, _, _ := newGradingTestService(t)
	repo.On("ResetApproval", mock.Anything, int64(5)).Return(nil)

	err := svc.ResetApproval(context.Background(), 5)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestResetApproval_RepoErrorPropagates(t *testing.T) {
	svc, repo, _, _ := newGradingTestService(t)
	repo.On("ResetApproval", mock.Anything, int64(5)).Return(errors.New("not approved"))

	err := svc.ResetApproval(context.Background(), 5)
	assert.Error(t, err)
}

func TestGetTestResults(t *testing.T) {
	svc, repo, _, _ := newGradingTestService(t)
	results := []*TestResult{
		{ID: 1, SubmissionID: 10, CommitSHA: "abc1234", Score: 100},
		{ID: 2, SubmissionID: 10, CommitSHA: "def5678", Score: 0},
	}
	repo.On("GetTestResultsBySubmission", mock.Anything, int64(10)).Return(results, nil)

	got, err := svc.GetTestResults(context.Background(), 10)
	assert.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestGetLatestTestResult(t *testing.T) {
	svc, repo, _, _ := newGradingTestService(t)
	tr := &TestResult{ID: 5, SubmissionID: 10, CommitSHA: "abc1234", Score: 100}
	repo.On("GetLatestTestResult", mock.Anything, int64(10)).Return(tr, nil)

	got, err := svc.GetLatestTestResult(context.Background(), 10)
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, int64(5), got.ID)
}
