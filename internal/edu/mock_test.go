package edu

import (
	"context"

	"forgejo.org/models/organization"
	"forgejo.org/models/perm"
	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"
	"github.com/stretchr/testify/mock"
)

// MockRepository is a mock implementation of Repository
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) CreateAssignment(ctx context.Context, assignment *Assignment) error {
	args := m.Called(ctx, assignment)
	return args.Error(0)
}

func (m *MockRepository) GetAssignmentByID(ctx context.Context, id int64) (*Assignment, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Assignment), args.Error(1)
}

func (m *MockRepository) GetAssignmentByCourseAndTask(ctx context.Context, courseID int64, taskName string) (*Assignment, error) {
	args := m.Called(ctx, courseID, taskName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Assignment), args.Error(1)
}

func (m *MockRepository) CreateSubmission(ctx context.Context, submission *Submission) error {
	args := m.Called(ctx, submission)
	return args.Error(0)
}

func (m *MockRepository) GetSubmission(ctx context.Context, assignmentID, userID int64) (*Submission, error) {
	args := m.Called(ctx, assignmentID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Submission), args.Error(1)
}

func (m *MockRepository) GetSubmissionByID(ctx context.Context, id int64) (*Submission, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Submission), args.Error(1)
}

func (m *MockRepository) GetSubmissionByEnrollmentAssignment(ctx context.Context, enrollmentID, assignmentID int64) (*Submission, error) {
	args := m.Called(ctx, enrollmentID, assignmentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Submission), args.Error(1)
}

func (m *MockRepository) UpdateSubmission(ctx context.Context, submission *Submission) error {
	args := m.Called(ctx, submission)
	return args.Error(0)
}

func (m *MockRepository) GetSubmissions(ctx context.Context, assignmentID int64) ([]*Submission, error) {
	args := m.Called(ctx, assignmentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Submission), args.Error(1)
}

func (m *MockRepository) GetAssignmentsForUser(ctx context.Context, userID int64) ([]*Assignment, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Assignment), args.Error(1)
}

func (m *MockRepository) UpdateAssignment(ctx context.Context, assignment *Assignment) error {
	args := m.Called(ctx, assignment)
	return args.Error(0)
}

func (m *MockRepository) DeleteAssignment(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepository) CreateCourse(ctx context.Context, course *Course) error {
	args := m.Called(ctx, course)
	return args.Error(0)
}

func (m *MockRepository) GetCourseByID(ctx context.Context, id int64) (*Course, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Course), args.Error(1)
}

func (m *MockRepository) GetCoursesByCreator(ctx context.Context, creatorID int64) ([]*Course, error) {
	args := m.Called(ctx, creatorID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Course), args.Error(1)
}

func (m *MockRepository) GetCoursesByUser(ctx context.Context, userID int64) ([]*Course, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Course), args.Error(1)
}

func (m *MockRepository) UpdateCourse(ctx context.Context, course *Course) error {
	args := m.Called(ctx, course)
	return args.Error(0)
}

func (m *MockRepository) DeleteCourse(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepository) GetAssignmentsByCourse(ctx context.Context, courseID int64) ([]*Assignment, error) {
	args := m.Called(ctx, courseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Assignment), args.Error(1)
}

func (m *MockRepository) EnrollUser(ctx context.Context, enrollment *CourseEnrollment) error {
	args := m.Called(ctx, enrollment)
	return args.Error(0)
}

func (m *MockRepository) GetEnrollment(ctx context.Context, courseID, userID int64) (*CourseEnrollment, error) {
	args := m.Called(ctx, courseID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*CourseEnrollment), args.Error(1)
}

func (m *MockRepository) GetEnrollmentByCourseUser(ctx context.Context, courseID, userID int64) (*CourseEnrollment, error) {
	args := m.Called(ctx, courseID, userID)
	v, _ := args.Get(0).(*CourseEnrollment)
	return v, args.Error(1)
}

func (m *MockRepository) GetEnrollments(ctx context.Context, courseID int64) ([]*CourseEnrollment, error) {
	args := m.Called(ctx, courseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*CourseEnrollment), args.Error(1)
}

func (m *MockRepository) RemoveEnrollment(ctx context.Context, courseID, userID int64) error {
	args := m.Called(ctx, courseID, userID)
	return args.Error(0)
}

func (m *MockRepository) UpdateEnrollment(ctx context.Context, e *CourseEnrollment) error {
	args := m.Called(ctx, e)
	return args.Error(0)
}

func (m *MockRepository) CreateImportDraft(ctx context.Context, draft *ImportDraft) error {
	args := m.Called(ctx, draft)
	return args.Error(0)
}

func (m *MockRepository) GetImportDraft(ctx context.Context, id int64) (*ImportDraft, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ImportDraft), args.Error(1)
}

func (m *MockRepository) CreateImportDraftRows(ctx context.Context, rows []*ImportDraftRow) error {
	args := m.Called(ctx, rows)
	return args.Error(0)
}

func (m *MockRepository) GetImportDraftRows(ctx context.Context, draftID int64) ([]*ImportDraftRow, error) {
	args := m.Called(ctx, draftID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ImportDraftRow), args.Error(1)
}

func (m *MockRepository) UpdateImportDraftRow(ctx context.Context, row *ImportDraftRow) error {
	args := m.Called(ctx, row)
	return args.Error(0)
}

func (m *MockRepository) UpdateImportDraft(ctx context.Context, draft *ImportDraft) error {
	args := m.Called(ctx, draft)
	return args.Error(0)
}

func (m *MockRepository) DeleteImportDraft(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepository) CreateInitForksTask(ctx context.Context, task *InitForksTask) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockRepository) GetInitForksTask(ctx context.Context, id int64) (*InitForksTask, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*InitForksTask), args.Error(1)
}

func (m *MockRepository) GetInitForksTaskByCourse(ctx context.Context, courseID int64) (*InitForksTask, error) {
	args := m.Called(ctx, courseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*InitForksTask), args.Error(1)
}

func (m *MockRepository) UpdateInitForksTask(ctx context.Context, task *InitForksTask) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockRepository) CreateTestResult(ctx context.Context, tr *TestResult) error {
	args := m.Called(ctx, tr)
	return args.Error(0)
}

func (m *MockRepository) GetTestResultsBySubmission(ctx context.Context, submissionID int64) ([]*TestResult, error) {
	args := m.Called(ctx, submissionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*TestResult), args.Error(1)
}

func (m *MockRepository) GetLatestTestResult(ctx context.Context, submissionID int64) (*TestResult, error) {
	args := m.Called(ctx, submissionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*TestResult), args.Error(1)
}

func (m *MockRepository) GradeSubmission(ctx context.Context, submissionID int64, grade int, comment string, gradedByID int64) error {
	args := m.Called(ctx, submissionID, grade, comment, gradedByID)
	return args.Error(0)
}

func (m *MockRepository) AutoGradeSubmission(ctx context.Context, submissionID int64, grade int) error {
	args := m.Called(ctx, submissionID, grade)
	return args.Error(0)
}

func (m *MockRepository) ResetToAutoGrade(ctx context.Context, submissionID int64, grade int) error {
	args := m.Called(ctx, submissionID, grade)
	return args.Error(0)
}

func (m *MockRepository) CreateDistributeTask(ctx context.Context, t *DistributeTask) error {
	args := m.Called(ctx, t)
	return args.Error(0)
}

func (m *MockRepository) GetDistributeTask(ctx context.Context, id int64) (*DistributeTask, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*DistributeTask), args.Error(1)
}

func (m *MockRepository) UpdateDistributeTask(ctx context.Context, t *DistributeTask) error {
	args := m.Called(ctx, t)
	return args.Error(0)
}

func (m *MockRepository) GetDistributeTaskByAssignment(ctx context.Context, assignmentID int64) (*DistributeTask, error) {
	args := m.Called(ctx, assignmentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*DistributeTask), args.Error(1)
}

func (m *MockRepository) CreateCourseSyncTask(ctx context.Context, task *CourseSyncTask) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockRepository) GetCourseSyncTask(ctx context.Context, id int64) (*CourseSyncTask, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*CourseSyncTask), args.Error(1)
}

func (m *MockRepository) GetCourseSyncTaskByCourse(ctx context.Context, courseID int64) (*CourseSyncTask, error) {
	args := m.Called(ctx, courseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*CourseSyncTask), args.Error(1)
}

func (m *MockRepository) UpdateCourseSyncTask(ctx context.Context, task *CourseSyncTask) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockRepository) CreateCourseSyncPR(ctx context.Context, p *CourseSyncPR) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *MockRepository) UpdateCourseSyncPR(ctx context.Context, p *CourseSyncPR) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *MockRepository) ListCourseSyncPRsByTask(ctx context.Context, taskID int64) ([]*CourseSyncPR, error) {
	args := m.Called(ctx, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*CourseSyncPR), args.Error(1)
}

func (m *MockRepository) GetCourseSyncPR(ctx context.Context, id int64) (*CourseSyncPR, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*CourseSyncPR), args.Error(1)
}

// MockUserCreator mocks the UserCreator interface
type MockUserCreator struct {
	mock.Mock
}

func (m *MockUserCreator) CreateUser(ctx context.Context, username, email, password, fullName string) error {
	args := m.Called(ctx, username, email, password, fullName)
	return args.Error(0)
}

func (m *MockUserCreator) GetUserByName(ctx context.Context, name string) (*user_model.User, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*user_model.User), args.Error(1)
}

func (m *MockUserCreator) GetUserByID(ctx context.Context, id int64) (*user_model.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*user_model.User), args.Error(1)
}

func (m *MockUserCreator) GetUserByEmail(ctx context.Context, email string) (*user_model.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*user_model.User), args.Error(1)
}

// MockRepoForker mocks the RepoForker interface
type MockRepoForker struct {
	mock.Mock
}

func (m *MockRepoForker) ForkRepositoryAndUpdates(ctx context.Context, doer, owner *user_model.User, opts ForkRepoOptions) (*repo_model.Repository, error) {
	args := m.Called(ctx, doer, owner, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repo_model.Repository), args.Error(1)
}

func (m *MockRepoForker) GetRepositoryByID(ctx context.Context, id int64) (*repo_model.Repository, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repo_model.Repository), args.Error(1)
}

func (m *MockRepoForker) SyncFork(ctx context.Context, doer *user_model.User, forkRepo *repo_model.Repository, branch string) error {
	args := m.Called(ctx, doer, forkRepo, branch)
	return args.Error(0)
}

func (m *MockRepoForker) GetDefaultBranch(ctx context.Context, repoID int64) (string, error) {
	args := m.Called(ctx, repoID)
	return args.String(0), args.Error(1)
}

func (m *MockRepoForker) AddCollaborator(ctx context.Context, repoID, userID int64, mode perm.AccessMode) error {
	args := m.Called(ctx, repoID, userID, mode)
	return args.Error(0)
}

func (m *MockRepoForker) RemoveCollaborator(ctx context.Context, repoID, userID int64) error {
	args := m.Called(ctx, repoID, userID)
	return args.Error(0)
}

func (m *MockRepoForker) ProtectMainBranch(ctx context.Context, repoID int64, branchName string) error {
	args := m.Called(ctx, repoID, branchName)
	return args.Error(0)
}

func (m *MockRepoForker) GetRepositoryByOwnerAndName(ctx context.Context, ownerID int64, repoName string) (*repo_model.Repository, error) {
	args := m.Called(ctx, ownerID, repoName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repo_model.Repository), args.Error(1)
}

func (m *MockRepoForker) BranchExists(ctx context.Context, repoID int64, branchName string) (bool, error) {
	args := m.Called(ctx, repoID, branchName)
	return args.Bool(0), args.Error(1)
}

// MockOrgManager mocks the OrgManager interface
type MockOrgManager struct {
	mock.Mock
}

func (m *MockOrgManager) EnsureTeam(ctx context.Context, orgID int64, teamName string, accessMode perm.AccessMode) (*organization.Team, error) {
	args := m.Called(ctx, orgID, teamName, accessMode)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*organization.Team), args.Error(1)
}

func (m *MockOrgManager) AddTeamMember(ctx context.Context, team *organization.Team, userID int64) error {
	args := m.Called(ctx, team, userID)
	return args.Error(0)
}

func (m *MockOrgManager) RemoveTeamMember(ctx context.Context, team *organization.Team, userID int64) error {
	args := m.Called(ctx, team, userID)
	return args.Error(0)
}

func (m *MockOrgManager) GetTeam(ctx context.Context, orgID int64, name string) (*organization.Team, error) {
	args := m.Called(ctx, orgID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*organization.Team), args.Error(1)
}
