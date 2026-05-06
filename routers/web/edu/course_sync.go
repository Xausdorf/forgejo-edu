package edu

import (
	"errors"
	"net/http"
	"strconv"

	"forgejo.org/internal/edu"
	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/base"
	"forgejo.org/modules/log"
	"forgejo.org/modules/setting"
	"forgejo.org/services/context"
)

const (
	tplCourseSync base.TplName = "edu/course_sync"
)

// On failure the response has already been written.
func loadCourseForSync(ctx *context.Context) (*edu.Course, bool) {
	if !isFullTeacher(ctx) {
		ctx.Error(http.StatusForbidden, "Only teachers can manage course sync")
		return nil, false
	}
	courseID := ctx.ParamsInt64(":id")
	svc := edu.GetService()
	if svc == nil {
		ctx.ServerError("GetService", nil)
		return nil, false
	}
	course, err := svc.GetCourseByID(ctx, courseID)
	if err != nil {
		ctx.ServerError("GetCourseByID", err)
		return nil, false
	}
	if course == nil {
		ctx.NotFound("Course not found", nil)
		return nil, false
	}
	if course.CreatorID != ctx.Doer.ID && !ctx.Doer.IsAdmin {
		ctx.Error(http.StatusForbidden, "You can only manage your own courses")
		return nil, false
	}
	return course, true
}

func courseSyncURL(ctx *context.Context) string {
	return setting.AppSubURL + "/edu/teacher/courses/" + ctx.Params(":id") + "/sync"
}

func CourseSyncPage(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("edu.sync.title")
	ctx.Data["PageIsEduCourses"] = true

	course, ok := loadCourseForSync(ctx)
	if !ok {
		return
	}
	ctx.Data["Course"] = course

	svc := edu.GetService()
	task, err := svc.GetCourseSyncTaskByCourse(ctx, course.ID)
	if err != nil {
		ctx.ServerError("GetCourseSyncTaskByCourse", err)
		return
	}
	ctx.Data["SyncTask"] = task

	if task != nil {
		prs, err := svc.ListCourseSyncPRsByTask(ctx, task.ID)
		if err != nil {
			ctx.ServerError("ListCourseSyncPRsByTask", err)
			return
		}
		ctx.Data["SyncPRs"] = prs

		enrollments, _ := svc.GetEnrollments(ctx, course.ID)
		userByEnrollment := make(map[int64]*user_model.User, len(enrollments))
		repoByEnrollment := make(map[int64]*repo_model.Repository, len(enrollments))
		orgUser, _ := user_model.GetUserByID(ctx, course.OrgID)
		for _, e := range enrollments {
			if u, err := user_model.GetUserByID(ctx, e.UserID); err == nil {
				userByEnrollment[e.ID] = u
			}
			if e.StudentForkRepoID != 0 {
				if r, err := repo_model.GetRepositoryByID(ctx, e.StudentForkRepoID); err == nil {
					repoByEnrollment[e.ID] = r
				}
			}
		}
		ctx.Data["UserByEnrollment"] = userByEnrollment
		ctx.Data["RepoByEnrollment"] = repoByEnrollment
		ctx.Data["OrgUser"] = orgUser
	}

	setEduNavContext(ctx)
	ctx.HTML(http.StatusOK, tplCourseSync)
}

func StartCourseSyncPost(ctx *context.Context) {
	course, ok := loadCourseForSync(ctx)
	if !ok {
		return
	}
	if _, err := edu.GetService().StartCourseSync(ctx, course.ID, ctx.Doer.ID); err != nil {
		switch {
		case errors.Is(err, edu.ErrCourseSyncNoMaster):
			ctx.Flash.Error(ctx.Tr("edu.sync.no_master"))
		case errors.Is(err, edu.ErrCourseSyncNoOrg):
			ctx.Flash.Error(ctx.Tr("edu.sync.no_org"))
		case errors.Is(err, edu.ErrCourseSyncNoInit):
			ctx.Flash.Error(ctx.Tr("edu.sync.no_init"))
		default:
			ctx.ServerError("StartCourseSync", err)
			return
		}
		ctx.Redirect(courseSyncURL(ctx))
		return
	}
	ctx.Flash.Success(ctx.Tr("edu.sync.started"))
	ctx.Redirect(courseSyncURL(ctx))
}

func CourseSyncStatus(ctx *context.Context) {
	course, ok := loadCourseForSync(ctx)
	if !ok {
		return
	}
	task, err := edu.GetService().GetCourseSyncTaskByCourse(ctx, course.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if task == nil {
		ctx.JSON(http.StatusOK, map[string]any{"status": "none"})
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{
		"id":        task.ID,
		"status":    task.Status,
		"total":     task.TotalRepos,
		"synced":    task.Synced,
		"skipped":   task.Skipped,
		"failed":    task.Failed,
		"error_log": task.ErrorLog,
		"updated":   task.UpdatedUnix,
	})
}

func MergeAllCourseSyncPost(ctx *context.Context) {
	_, ok := loadCourseForSync(ctx)
	if !ok {
		return
	}
	taskID := ctx.ParamsInt64(":taskID")
	merged, err := edu.GetService().MergeAllCourseSyncPRs(ctx, taskID, ctx.Doer.ID)
	if err != nil {
		log.Error("merge-all course-sync (task %d): %v", taskID, err)
		ctx.Flash.Error(err.Error())
		ctx.Redirect(courseSyncURL(ctx))
		return
	}
	ctx.Flash.Success(string(ctx.Tr("edu.sync.merge_all_success")) + " (" + strconv.Itoa(merged) + ")")
	ctx.Redirect(courseSyncURL(ctx))
}

func MergeOneCourseSyncPost(ctx *context.Context) {
	_, ok := loadCourseForSync(ctx)
	if !ok {
		return
	}
	syncPRID := ctx.ParamsInt64(":prID")
	if err := edu.GetService().MergeCourseSyncPR(ctx, syncPRID, ctx.Doer.ID); err != nil {
		log.Error("merge course-sync pr %d: %v", syncPRID, err)
		switch {
		case errors.Is(err, edu.ErrCourseSyncPRNotFound):
			ctx.NotFound("Sync PR not found", nil)
			return
		case errors.Is(err, edu.ErrCourseSyncPRNotPending):
			ctx.Flash.Error(err.Error())
		default:
			ctx.Flash.Error(ctx.Tr("edu.sync.merge_failed", err.Error()))
		}
		ctx.Redirect(courseSyncURL(ctx))
		return
	}
	ctx.Flash.Success(ctx.Tr("edu.sync.merge_one_success"))
	ctx.Redirect(courseSyncURL(ctx))
}
