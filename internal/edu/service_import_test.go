package edu

import (
	"context"
	"fmt"
	"testing"

	user_model "forgejo.org/models/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUploadCSV(t *testing.T) {
	mockRepo := new(MockRepository)
	mockForker := new(MockRepoForker)
	svc := NewService(mockRepo, mockForker)

	csvData := []byte("FullName;Email\nIvanov Ivan;ivan@test.com\nPetrova Anna;anna@test.com")
	mapping := CSVColumnMapping{
		FullNameCol: 0,
		EmailCol:    1,
		GroupCol:    -1,
		HasHeader:   true,
	}

	mockRepo.On("CreateImportDraft", mock.Anything, mock.AnythingOfType("*edu.ImportDraft")).
		Run(func(args mock.Arguments) {
			d := args.Get(1).(*ImportDraft)
			d.ID = 1
		}).
		Return(nil)

	mockRepo.On("CreateImportDraftRows", mock.Anything, mock.AnythingOfType("[]*edu.ImportDraftRow")).
		Return(nil)

	draft, err := svc.UploadCSV(context.Background(), 10, 1, csvData, mapping)
	assert.NoError(t, err)
	assert.NotNil(t, draft)
	assert.Equal(t, int64(1), draft.ID)
	assert.Equal(t, int64(10), draft.CourseID)
	assert.Equal(t, "draft", draft.Status)

	mockRepo.AssertExpectations(t)
}

func TestUploadCSV_EmptyCSV(t *testing.T) {
	mockRepo := new(MockRepository)
	mockForker := new(MockRepoForker)
	svc := NewService(mockRepo, mockForker)

	csvData := []byte("")
	mapping := CSVColumnMapping{FullNameCol: 0, EmailCol: -1, GroupCol: -1}

	_, err := svc.UploadCSV(context.Background(), 10, 1, csvData, mapping)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no valid rows")
}

func TestGetImportDraft(t *testing.T) {
	mockRepo := new(MockRepository)
	mockForker := new(MockRepoForker)
	svc := NewService(mockRepo, mockForker)

	draft := &ImportDraft{ID: 1, CourseID: 10, Status: "draft"}
	rows := []*ImportDraftRow{
		{ID: 1, DraftID: 1, FullName: "Ivanov Ivan", Username: "ivanov-i", Status: "pending"},
	}

	mockRepo.On("GetImportDraft", mock.Anything, int64(1)).Return(draft, nil)
	mockRepo.On("GetImportDraftRows", mock.Anything, int64(1)).Return(rows, nil)

	gotDraft, gotRows, err := svc.GetImportDraft(context.Background(), 1)
	assert.NoError(t, err)
	assert.Equal(t, draft, gotDraft)
	assert.Equal(t, rows, gotRows)

	mockRepo.AssertExpectations(t)
}

func TestGetImportDraft_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	mockForker := new(MockRepoForker)
	svc := NewService(mockRepo, mockForker)

	mockRepo.On("GetImportDraft", mock.Anything, int64(99)).Return(nil, nil)

	gotDraft, gotRows, err := svc.GetImportDraft(context.Background(), 99)
	assert.NoError(t, err)
	assert.Nil(t, gotDraft)
	assert.Nil(t, gotRows)
}

func TestUpdateDraftRow(t *testing.T) {
	mockRepo := new(MockRepository)
	mockForker := new(MockRepoForker)
	svc := NewService(mockRepo, mockForker)

	mockRepo.On("UpdateImportDraftRow", mock.Anything, mock.MatchedBy(func(row *ImportDraftRow) bool {
		return row.ID == 5 && row.Username == "new-user" && row.Email == "new@test.com" && row.Group == "" && row.Status == "pending"
	})).Return(nil)

	err := svc.UpdateDraftRow(context.Background(), 5, "new-user", "new@test.com", "")
	assert.NoError(t, err)

	mockRepo.AssertExpectations(t)
}

func TestUpdateDraftRow_WithGroup(t *testing.T) {
	mockRepo := new(MockRepository)
	mockForker := new(MockRepoForker)
	svc := NewService(mockRepo, mockForker)

	mockRepo.On("UpdateImportDraftRow", mock.Anything, mock.MatchedBy(func(row *ImportDraftRow) bool {
		return row.ID == 5 && row.Username == "new-user" && row.Email == "new@test.com" && row.Group == "SE-241" && row.Status == "pending"
	})).Return(nil)

	err := svc.UpdateDraftRow(context.Background(), 5, "new-user", "new@test.com", "SE-241")
	assert.NoError(t, err)

	mockRepo.AssertExpectations(t)
}

func TestExecuteImport_CreateNewUsers(t *testing.T) {
	mockRepo := new(MockRepository)
	mockForker := new(MockRepoForker)
	mockUsers := new(MockUserCreator)
	svc := NewService(mockRepo, mockForker, mockUsers)

	draft := &ImportDraft{ID: 1, CourseID: 10, Status: "draft"}
	rows := []*ImportDraftRow{
		{ID: 1, DraftID: 1, FullName: "Ivanov Ivan", Username: "ivanov-i", Status: "pending"},
		{ID: 2, DraftID: 1, FullName: "Petrova Anna", Username: "petrova-a", Email: "anna@test.com", Status: "pending"},
	}

	mockRepo.On("GetImportDraft", mock.Anything, int64(1)).Return(draft, nil)
	mockRepo.On("GetImportDraftRows", mock.Anything, int64(1)).Return(rows, nil)

	// Email check: petrova has email, not found in system
	mockUsers.On("GetUserByEmail", mock.Anything, "anna@test.com").Return(nil, fmt.Errorf("not found"))

	// Username check: neither exists — first call from existing-user check, second from resolveUniqueUsername
	mockUsers.On("GetUserByName", mock.Anything, "ivanov-i").Return(nil, fmt.Errorf("not found")).Once()  // existing check
	mockUsers.On("GetUserByName", mock.Anything, "petrova-a").Return(nil, fmt.Errorf("not found")).Once() // existing check
	// resolveUniqueUsername calls (username is free so returns immediately)
	mockUsers.On("GetUserByName", mock.Anything, "ivanov-i").Return(nil, fmt.Errorf("not found")).Once()
	mockUsers.On("GetUserByName", mock.Anything, "petrova-a").Return(nil, fmt.Errorf("not found")).Once()

	mockUsers.On("CreateUser", mock.Anything, "ivanov-i", "ivanov-i@edu.local", mock.AnythingOfType("string"), "Ivanov Ivan").Return(nil)
	mockUsers.On("CreateUser", mock.Anything, "petrova-a", "anna@test.com", mock.AnythingOfType("string"), "Petrova Anna").Return(nil)

	// After creation, GetUserByName returns the new user
	mockUsers.On("GetUserByName", mock.Anything, "ivanov-i").Return(&user_model.User{ID: 100, Name: "ivanov-i"}, nil)
	mockUsers.On("GetUserByName", mock.Anything, "petrova-a").Return(&user_model.User{ID: 101, Name: "petrova-a"}, nil)

	mockRepo.On("EnrollUser", mock.Anything, mock.AnythingOfType("*edu.CourseEnrollment")).Return(nil).Times(2)
	mockRepo.On("UpdateImportDraftRow", mock.Anything, mock.AnythingOfType("*edu.ImportDraftRow")).Return(nil)
	mockRepo.On("UpdateImportDraft", mock.Anything, mock.AnythingOfType("*edu.ImportDraft")).Return(nil)

	result, err := svc.ExecuteImport(context.Background(), 1, 1, RoleStudent)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, result.TotalRows)
	assert.Equal(t, 2, result.Created)
	assert.Equal(t, 0, result.AlreadyExist)
	assert.Equal(t, 0, result.Errors)
	assert.Len(t, result.Credentials, 2)
}

func TestExecuteImport_ExistingUser(t *testing.T) {
	mockRepo := new(MockRepository)
	mockForker := new(MockRepoForker)
	mockUsers := new(MockUserCreator)
	svc := NewService(mockRepo, mockForker, mockUsers)

	draft := &ImportDraft{ID: 1, CourseID: 10, Status: "draft"}
	rows := []*ImportDraftRow{
		{ID: 1, DraftID: 1, FullName: "Ivanov Ivan", Username: "ivanov-i", Status: "pending"},
	}

	mockRepo.On("GetImportDraft", mock.Anything, int64(1)).Return(draft, nil)
	mockRepo.On("GetImportDraftRows", mock.Anything, int64(1)).Return(rows, nil)

	// User already exists by username (no email to check)
	mockUsers.On("GetUserByName", mock.Anything, "ivanov-i").Return(&user_model.User{ID: 50, Name: "ivanov-i"}, nil)

	mockRepo.On("EnrollUser", mock.Anything, mock.AnythingOfType("*edu.CourseEnrollment")).Return(nil)
	mockRepo.On("UpdateImportDraftRow", mock.Anything, mock.AnythingOfType("*edu.ImportDraftRow")).Return(nil)
	mockRepo.On("UpdateImportDraft", mock.Anything, mock.AnythingOfType("*edu.ImportDraft")).Return(nil)

	result, err := svc.ExecuteImport(context.Background(), 1, 1, RoleStudent)
	assert.NoError(t, err)
	assert.Equal(t, 1, result.TotalRows)
	assert.Equal(t, 0, result.Created)
	assert.Equal(t, 1, result.AlreadyExist)
	assert.Equal(t, 0, result.Errors)
}

func TestExecuteImport_InvalidUsername(t *testing.T) {
	mockRepo := new(MockRepository)
	mockForker := new(MockRepoForker)
	mockUsers := new(MockUserCreator)
	svc := NewService(mockRepo, mockForker, mockUsers)

	draft := &ImportDraft{ID: 1, CourseID: 10, Status: "draft"}
	rows := []*ImportDraftRow{
		{ID: 1, DraftID: 1, FullName: "Bad User", Username: "-invalid", Status: "pending"},
	}

	mockRepo.On("GetImportDraft", mock.Anything, int64(1)).Return(draft, nil)
	mockRepo.On("GetImportDraftRows", mock.Anything, int64(1)).Return(rows, nil)
	mockRepo.On("UpdateImportDraftRow", mock.Anything, mock.AnythingOfType("*edu.ImportDraftRow")).Return(nil)
	mockRepo.On("UpdateImportDraft", mock.Anything, mock.AnythingOfType("*edu.ImportDraft")).Return(nil)

	result, err := svc.ExecuteImport(context.Background(), 1, 1, RoleStudent)
	assert.NoError(t, err)
	assert.Equal(t, 1, result.TotalRows)
	assert.Equal(t, 0, result.Created)
	assert.Equal(t, 1, result.Errors)
}

func TestExecuteImport_NoUserCreator(t *testing.T) {
	mockRepo := new(MockRepository)
	mockForker := new(MockRepoForker)
	svc := NewService(mockRepo, mockForker) // no UserCreator

	_, err := svc.ExecuteImport(context.Background(), 1, 1, RoleStudent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user creator not configured")
}

func TestExecuteImport_ExistingUserByEmail(t *testing.T) {
	mockRepo := new(MockRepository)
	mockForker := new(MockRepoForker)
	mockUsers := new(MockUserCreator)
	svc := NewService(mockRepo, mockForker, mockUsers)

	draft := &ImportDraft{ID: 1, CourseID: 10, Status: "draft"}
	rows := []*ImportDraftRow{
		{ID: 1, DraftID: 1, FullName: "Ivanov Ivan", Username: "ivanov-i", Email: "ivan@uni.edu", Status: "pending"},
	}

	mockRepo.On("GetImportDraft", mock.Anything, int64(1)).Return(draft, nil)
	mockRepo.On("GetImportDraftRows", mock.Anything, int64(1)).Return(rows, nil)

	// User found by email (different username in system)
	mockUsers.On("GetUserByEmail", mock.Anything, "ivan@uni.edu").
		Return(&user_model.User{ID: 42, Name: "old-username"}, nil)

	mockRepo.On("EnrollUser", mock.Anything, mock.MatchedBy(func(e *CourseEnrollment) bool {
		return e.UserID == 42 && e.CourseID == 10
	})).Return(nil)
	mockRepo.On("UpdateImportDraftRow", mock.Anything, mock.AnythingOfType("*edu.ImportDraftRow")).Return(nil)
	mockRepo.On("UpdateImportDraft", mock.Anything, mock.AnythingOfType("*edu.ImportDraft")).Return(nil)

	result, err := svc.ExecuteImport(context.Background(), 1, 1, RoleStudent)
	assert.NoError(t, err)
	assert.Equal(t, 1, result.TotalRows)
	assert.Equal(t, 0, result.Created)
	assert.Equal(t, 1, result.AlreadyExist)
	assert.Equal(t, 0, result.Errors)
}

func TestExecuteImport_DuplicateUsernamesInBatch(t *testing.T) {
	mockRepo := new(MockRepository)
	mockForker := new(MockRepoForker)
	mockUsers := new(MockUserCreator)
	svc := NewService(mockRepo, mockForker, mockUsers)

	draft := &ImportDraft{ID: 1, CourseID: 10, Status: "draft"}
	rows := []*ImportDraftRow{
		{ID: 1, DraftID: 1, FullName: "Ivanov Ivan Ivanovich", Username: "ivanov-ii", Status: "pending"},
		{ID: 2, DraftID: 1, FullName: "Ivanov Igor Igorevich", Username: "ivanov-ii", Status: "pending"},
	}

	mockRepo.On("GetImportDraft", mock.Anything, int64(1)).Return(draft, nil)
	mockRepo.On("GetImportDraftRows", mock.Anything, int64(1)).Return(rows, nil)

	// Row 1: username "ivanov-ii" not in system → create
	mockUsers.On("GetUserByName", mock.Anything, "ivanov-ii").Return(nil, fmt.Errorf("not found")).Once()  // existing check
	mockUsers.On("GetUserByName", mock.Anything, "ivanov-ii").Return(nil, fmt.Errorf("not found")).Once()  // resolveUniqueUsername
	mockUsers.On("CreateUser", mock.Anything, "ivanov-ii", "ivanov-ii@edu.local", mock.AnythingOfType("string"), "Ivanov Ivan Ivanovich").Return(nil)
	mockUsers.On("GetUserByName", mock.Anything, "ivanov-ii").Return(&user_model.User{ID: 100, Name: "ivanov-ii"}, nil) // after creation

	// Row 2: username "ivanov-ii" already in batch → resolveUniqueUsername tries "ivanov-ii2"
	mockUsers.On("GetUserByName", mock.Anything, "ivanov-ii2").Return(nil, fmt.Errorf("not found")).Once() // resolveUniqueUsername
	mockUsers.On("CreateUser", mock.Anything, "ivanov-ii2", "ivanov-ii2@edu.local", mock.AnythingOfType("string"), "Ivanov Igor Igorevich").Return(nil)
	mockUsers.On("GetUserByName", mock.Anything, "ivanov-ii2").Return(&user_model.User{ID: 101, Name: "ivanov-ii2"}, nil) // after creation

	mockRepo.On("EnrollUser", mock.Anything, mock.AnythingOfType("*edu.CourseEnrollment")).Return(nil).Times(2)
	mockRepo.On("UpdateImportDraftRow", mock.Anything, mock.AnythingOfType("*edu.ImportDraftRow")).Return(nil)
	mockRepo.On("UpdateImportDraft", mock.Anything, mock.AnythingOfType("*edu.ImportDraft")).Return(nil)

	result, err := svc.ExecuteImport(context.Background(), 1, 1, RoleStudent)
	assert.NoError(t, err)
	assert.Equal(t, 2, result.TotalRows)
	assert.Equal(t, 2, result.Created)
	assert.Equal(t, 0, result.AlreadyExist)
	assert.Equal(t, 0, result.Errors)
	assert.Len(t, result.Credentials, 2)
	assert.Equal(t, "ivanov-ii", result.Credentials[0].Username)
	assert.Equal(t, "ivanov-ii2", result.Credentials[1].Username)
}

func TestExecuteImport_UsernameExistsInSystem(t *testing.T) {
	mockRepo := new(MockRepository)
	mockForker := new(MockRepoForker)
	mockUsers := new(MockUserCreator)
	svc := NewService(mockRepo, mockForker, mockUsers)

	draft := &ImportDraft{ID: 1, CourseID: 10, Status: "draft"}
	rows := []*ImportDraftRow{
		{ID: 1, DraftID: 1, FullName: "Ivanov Ivan Ivanovich", Username: "ivanov-ii", Email: "new-ivan@test.com", Status: "pending"},
	}

	mockRepo.On("GetImportDraft", mock.Anything, int64(1)).Return(draft, nil)
	mockRepo.On("GetImportDraftRows", mock.Anything, int64(1)).Return(rows, nil)

	// Email not found → not an existing user
	mockUsers.On("GetUserByEmail", mock.Anything, "new-ivan@test.com").Return(nil, fmt.Errorf("not found"))

	// Username exists in system → enroll existing user (not create new with suffix)
	mockUsers.On("GetUserByName", mock.Anything, "ivanov-ii").Return(&user_model.User{ID: 50, Name: "ivanov-ii"}, nil)

	mockRepo.On("EnrollUser", mock.Anything, mock.MatchedBy(func(e *CourseEnrollment) bool {
		return e.UserID == 50 && e.CourseID == 10
	})).Return(nil)
	mockRepo.On("UpdateImportDraftRow", mock.Anything, mock.AnythingOfType("*edu.ImportDraftRow")).Return(nil)
	mockRepo.On("UpdateImportDraft", mock.Anything, mock.AnythingOfType("*edu.ImportDraft")).Return(nil)

	result, err := svc.ExecuteImport(context.Background(), 1, 1, RoleStudent)
	assert.NoError(t, err)
	assert.Equal(t, 1, result.TotalRows)
	assert.Equal(t, 0, result.Created)
	assert.Equal(t, 1, result.AlreadyExist)
	assert.Equal(t, 0, result.Errors)
}

func TestDeleteImportDraft(t *testing.T) {
	mockRepo := new(MockRepository)
	mockForker := new(MockRepoForker)
	svc := NewService(mockRepo, mockForker)

	mockRepo.On("DeleteImportDraft", mock.Anything, int64(5)).Return(nil)

	err := svc.DeleteImportDraft(context.Background(), 5)
	assert.NoError(t, err)

	mockRepo.AssertExpectations(t)
}
