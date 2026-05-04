package edu

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateAssignment_StoresTaskNameAndGlob(t *testing.T) {
	mockRepo := new(MockRepository)
	mockForker := new(MockRepoForker)
	service := NewService(mockRepo, mockForker)
	ctx := context.Background()

	mockRepo.On("GetCourseByID", ctx, int64(1)).Return(&Course{ID: 1, TasksMasterRepoID: 99}, nil)
	mockRepo.On("GetAssignmentByCourseAndTask", ctx, int64(1), "multiplication").Return(nil, nil)
	mockForker.On("BranchExists", ctx, int64(99), "submits/multiplication").Return(true, nil)
	mockRepo.On("CreateAssignment", ctx, mock.MatchedBy(func(a *Assignment) bool {
		return a.TaskName == "multiplication" && a.AllowedFilesGlob == "tasks/multiplication/*.cpp"
	})).Return(nil).Once()

	a, err := service.CreateAssignment(ctx, CreateAssignmentOptions{
		CourseID:         1,
		TaskName:         "multiplication",
		AllowedFilesGlob: "tasks/multiplication/*.cpp",
		Title:            "Умножение",
	})
	assert.NoError(t, err)
	assert.Equal(t, "multiplication", a.TaskName)
}

func TestGetAssignmentsForUser_Service(t *testing.T) {
	mockRepo := new(MockRepository)
	mockForker := new(MockRepoForker)
	svc := NewService(mockRepo, mockForker)
	ctx := context.Background()

	expected := []*Assignment{{ID: 1, CourseID: 10, Title: "HW1"}}
	mockRepo.On("GetAssignmentsForUser", ctx, int64(5)).Return(expected, nil)

	result, err := svc.GetAssignmentsForUser(ctx, 5)
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
	mockRepo.AssertExpectations(t)
}

func TestUpdateAssignment_Service(t *testing.T) {
	mockRepo := new(MockRepository)
	mockForker := new(MockRepoForker)
	svc := NewService(mockRepo, mockForker)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		a := &Assignment{ID: 1, Title: "Updated"}
		mockRepo.On("UpdateAssignment", ctx, mock.MatchedBy(func(a *Assignment) bool {
			return a.ID == int64(1) && a.Title == "Updated" && a.UpdatedUnix > 0
		})).Return(nil).Once()

		err := svc.UpdateAssignment(ctx, a)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Empty title", func(t *testing.T) {
		a := &Assignment{ID: 1, Title: ""}
		err := svc.UpdateAssignment(ctx, a)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "title is required")
	})
}

func TestDeleteAssignment_Service(t *testing.T) {
	mockRepo := new(MockRepository)
	mockForker := new(MockRepoForker)
	svc := NewService(mockRepo, mockForker)
	ctx := context.Background()

	mockRepo.On("DeleteAssignment", ctx, int64(1)).Return(nil)

	err := svc.DeleteAssignment(ctx, 1)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

