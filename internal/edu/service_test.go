package edu

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateAssignment(t *testing.T) {
	mockRepo := new(MockRepository)
	mockForker := new(MockRepoForker)
	service := NewService(mockRepo, mockForker)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		opts := CreateAssignmentOptions{
			CourseID:         1,
			TaskName:         "hw1",
			AllowedFilesGlob: "tasks/hw1/*.cpp",
			Title:            "Test Assignment",
			Description:      "Description",
			DeadlineUnix:     time.Now().Add(24 * time.Hour).Unix(),
		}

		mockRepo.On("GetCourseByID", ctx, int64(1)).Return(&Course{ID: 1, TasksMasterRepoID: 99}, nil)
		mockRepo.On("GetAssignmentByCourseAndTask", ctx, int64(1), "hw1").Return(nil, nil)
		mockForker.On("BranchExists", ctx, int64(99), "submits/hw1").Return(true, nil)
		mockRepo.On("CreateAssignment", ctx, mock.MatchedBy(func(a *Assignment) bool {
			return a.CourseID == opts.CourseID && a.TaskName == opts.TaskName && a.Title == opts.Title
		})).Return(nil)

		assignment, err := service.CreateAssignment(ctx, opts)
		assert.NoError(t, err)
		assert.NotNil(t, assignment)
		assert.Equal(t, opts.Title, assignment.Title)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Validation Error", func(t *testing.T) {
		opts := CreateAssignmentOptions{CourseID: 1}
		_, err := service.CreateAssignment(ctx, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "course_id, task_name and title are required")
	})
}

func TestGetSubmissions(t *testing.T) {
	mockRepo := new(MockRepository)
	mockForker := new(MockRepoForker)
	service := NewService(mockRepo, mockForker)
	ctx := context.Background()

	assignmentID := int64(100)
	expected := []*Submission{{ID: 1, Status: StatusSubmissionPending}}

	mockRepo.On("GetSubmissions", ctx, assignmentID).Return(expected, nil)

	result, err := service.GetSubmissions(ctx, assignmentID)
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
	mockRepo.AssertExpectations(t)
}
