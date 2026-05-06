package edu

import (
	"errors"
	"net/http"
	"time"

	"forgejo.org/internal/edu"
	org_model "forgejo.org/models/organization"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/base"
	"forgejo.org/modules/log"
	"forgejo.org/modules/setting"
	"forgejo.org/services/context"
)

const (
	tplCourseList   base.TplName = "edu/course_list"
	tplCourseDetail base.TplName = "edu/course_detail"
	tplCourseForm   base.TplName = "edu/course_form"
)

func CourseList(ctx *context.Context) {
	ctx.Data["Title"] = "Courses"
	ctx.Data["PageIsEduCourses"] = true

	svc := edu.GetService()
	if svc == nil {
		ctx.ServerError("GetService", nil)
		return
	}

	courses, err := svc.GetCoursesForUser(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetCoursesForUser", err)
		return
	}

	ctx.Data["Courses"] = courses
	setEduNavContext(ctx)
	ctx.HTML(http.StatusOK, tplCourseList)
}

func CourseDetail(ctx *context.Context) {
	ctx.Data["Title"] = "Course Detail"
	ctx.Data["PageIsEduCourses"] = true

	courseID := ctx.ParamsInt64(":id")
	svc := edu.GetService()
	if svc == nil {
		ctx.ServerError("GetService", nil)
		return
	}

	course, err := svc.GetCourseByID(ctx, courseID)
	if err != nil {
		ctx.ServerError("GetCourseByID", err)
		return
	}
	if course == nil {
		ctx.NotFound("Course not found", nil)
		return
	}
	ctx.Data["Course"] = course

	enrollments, err := svc.GetEnrollments(ctx, courseID)
	if err != nil {
		log.Error("Failed to get enrollments: %v", err)
	}
	ctx.Data["Enrollments"] = enrollments

	// Load user info for enrollments
	if len(enrollments) > 0 {
		userIDs := make([]int64, 0, len(enrollments))
		for _, e := range enrollments {
			userIDs = append(userIDs, e.UserID)
		}
		users, err := user_model.GetUsersByIDs(ctx, userIDs)
		if err != nil {
			log.Error("Failed to get users: %v", err)
		} else {
			userMap := make(map[int64]*user_model.User)
			for _, u := range users {
				userMap[u.ID] = u
			}
			ctx.Data["UserMap"] = userMap
		}
	}

	assignments, err := svc.GetAssignmentsByCourse(ctx, courseID)
	if err != nil {
		log.Error("Failed to get assignments for course: %v", err)
	}
	ctx.Data["Assignments"] = assignments

	submissionCounts := make(map[int64]int)
	for _, a := range assignments {
		subs, err := svc.GetSubmissions(ctx, a.ID)
		if err != nil {
			log.Error("Failed to get submissions for assignment %d: %v", a.ID, err)
			continue
		}
		submissionCounts[a.ID] = len(subs)
	}
	ctx.Data["SubmissionCounts"] = submissionCounts

	setEduNavContext(ctx)
	ctx.HTML(http.StatusOK, tplCourseDetail)
}

// loadOrgsAndRepos populates Orgs and (if selectedOrgID > 0) OrgRepos for the course form.
func loadOrgsAndRepos(ctx *context.Context, selectedOrgID int64) {
	orgs, err := org_model.GetOrgsCanCreateRepoByUserID(ctx, ctx.Doer.ID)
	if err != nil {
		log.Error("Failed to get orgs for user: %v", err)
	}
	ctx.Data["Orgs"] = orgs
	ctx.Data["SelectedOrgID"] = selectedOrgID

	if selectedOrgID > 0 {
		repos, err := org_model.GetOrgRepositories(ctx, selectedOrgID)
		if err != nil {
			log.Error("Failed to get org repos for %d: %v", selectedOrgID, err)
		}
		ctx.Data["OrgRepos"] = repos
	}
}

func NewCourse(ctx *context.Context) {
	if !isFullTeacher(ctx) {
		ctx.Error(http.StatusForbidden, "Only teachers can create courses")
		return
	}
	ctx.Data["Title"] = "New Course"
	ctx.Data["PageIsEduCourses"] = true
	selectedOrgID := ctx.FormInt64("org_id")
	loadOrgsAndRepos(ctx, selectedOrgID)
	setEduNavContext(ctx)
	ctx.HTML(http.StatusOK, tplCourseForm)
}

func NewCoursePost(ctx *context.Context) {
	if !isFullTeacher(ctx) {
		ctx.Error(http.StatusForbidden, "Only teachers can create courses")
		return
	}
	ctx.Data["Title"] = "New Course"
	ctx.Data["PageIsEduCourses"] = true

	name := ctx.FormString("name")
	description := ctx.FormString("description")
	startDateStr := ctx.FormString("start_date")
	endDateStr := ctx.FormString("end_date")

	orgID := ctx.FormInt64("org_id")

	if name == "" {
		loadOrgsAndRepos(ctx, orgID)
		ctx.RenderWithErr("Name is required.", tplCourseForm, nil)
		return
	}
	if len(name) > 255 {
		loadOrgsAndRepos(ctx, orgID)
		ctx.RenderWithErr(ctx.Tr("edu.name_too_long"), tplCourseForm, nil)
		return
	}

	var startUnix, endUnix int64
	if startDateStr != "" {
		t, err := time.Parse("2006-01-02T15:04", startDateStr)
		if err != nil {
			log.Warn("Failed to parse start_date: %v", err)
		} else {
			startUnix = t.Unix()
		}
	}
	if endDateStr != "" {
		t, err := time.Parse("2006-01-02T15:04", endDateStr)
		if err != nil {
			log.Warn("Failed to parse end_date: %v", err)
		} else {
			endUnix = t.Unix()
		}
	}

	svc := edu.GetService()
	if svc == nil {
		ctx.ServerError("GetService", nil)
		return
	}

	opts := edu.CreateCourseOptions{
		Name:              name,
		Description:       description,
		OrgID:             orgID,
		TasksMasterRepoID: ctx.FormInt64("tasks_master_repo_id"),
		StartUnix:         startUnix,
		EndUnix:           endUnix,
	}

	_, err := svc.CreateCourse(ctx, ctx.Doer.ID, opts)
	if err != nil {
		ctx.ServerError("CreateCourse", err)
		return
	}

	ctx.Redirect(setting.AppSubURL + "/edu/teacher/courses")
}

func EditCourse(ctx *context.Context) {
	ctx.Data["Title"] = "Edit Course"
	ctx.Data["PageIsEduCourses"] = true

	courseID := ctx.ParamsInt64(":id")
	svc := edu.GetService()
	if svc == nil {
		ctx.ServerError("GetService", nil)
		return
	}

	course, err := svc.GetCourseByID(ctx, courseID)
	if err != nil {
		ctx.ServerError("GetCourseByID", err)
		return
	}
	if course == nil {
		ctx.NotFound("Course not found", nil)
		return
	}

	if course.CreatorID != ctx.Doer.ID && !ctx.Doer.IsAdmin {
		ctx.Error(http.StatusForbidden, "You can only edit your own courses")
		return
	}

	ctx.Data["Course"] = course
	selectedOrgID := course.OrgID
	if ctx.FormString("org_id") != "" {
		selectedOrgID = ctx.FormInt64("org_id")
	}
	loadOrgsAndRepos(ctx, selectedOrgID)
	setEduNavContext(ctx)
	ctx.HTML(http.StatusOK, tplCourseForm)
}

func EditCoursePost(ctx *context.Context) {
	ctx.Data["Title"] = "Edit Course"
	ctx.Data["PageIsEduCourses"] = true

	courseID := ctx.ParamsInt64(":id")
	svc := edu.GetService()
	if svc == nil {
		ctx.ServerError("GetService", nil)
		return
	}

	course, err := svc.GetCourseByID(ctx, courseID)
	if err != nil {
		ctx.ServerError("GetCourseByID", err)
		return
	}
	if course == nil {
		ctx.NotFound("Course not found", nil)
		return
	}

	if course.CreatorID != ctx.Doer.ID && !ctx.Doer.IsAdmin {
		ctx.Error(http.StatusForbidden, "You can only edit your own courses")
		return
	}

	course.Name = ctx.FormString("name")
	course.Description = ctx.FormString("description")
	course.OrgID = ctx.FormInt64("org_id")
	course.TasksMasterRepoID = ctx.FormInt64("tasks_master_repo_id")

	if course.Name == "" {
		ctx.Data["Course"] = course
		loadOrgsAndRepos(ctx, course.OrgID)
		ctx.RenderWithErr("Name is required.", tplCourseForm, nil)
		return
	}
	if len(course.Name) > 255 {
		ctx.Data["Course"] = course
		loadOrgsAndRepos(ctx, course.OrgID)
		ctx.RenderWithErr(ctx.Tr("edu.name_too_long"), tplCourseForm, nil)
		return
	}

	startDateStr := ctx.FormString("start_date")
	endDateStr := ctx.FormString("end_date")

	if startDateStr != "" {
		t, err := time.Parse("2006-01-02T15:04", startDateStr)
		if err == nil {
			course.StartUnix = t.Unix()
		}
	} else {
		course.StartUnix = 0
	}
	if endDateStr != "" {
		t, err := time.Parse("2006-01-02T15:04", endDateStr)
		if err == nil {
			course.EndUnix = t.Unix()
		}
	} else {
		course.EndUnix = 0
	}

	if err := svc.UpdateCourse(ctx, course); err != nil {
		ctx.ServerError("UpdateCourse", err)
		return
	}

	ctx.Redirect(setting.AppSubURL + "/edu/teacher/courses/" + ctx.Params(":id"))
}

func DeleteCoursePost(ctx *context.Context) {
	courseID := ctx.ParamsInt64(":id")
	svc := edu.GetService()
	if svc == nil {
		ctx.ServerError("GetService", nil)
		return
	}

	course, err := svc.GetCourseByID(ctx, courseID)
	if err != nil {
		ctx.ServerError("GetCourseByID", err)
		return
	}
	if course == nil {
		ctx.NotFound("Course not found", nil)
		return
	}

	if course.CreatorID != ctx.Doer.ID && !ctx.Doer.IsAdmin {
		ctx.Error(http.StatusForbidden, "You can only delete your own courses")
		return
	}

	if err := svc.DeleteCourse(ctx, courseID); err != nil {
		ctx.ServerError("DeleteCourse", err)
		return
	}

	ctx.Redirect(setting.AppSubURL + "/edu/teacher/courses")
}

func EnrollUserPost(ctx *context.Context) {
	courseID := ctx.ParamsInt64(":id")
	username := ctx.FormString("username")
	role := ctx.FormString("role")
	groupName := ctx.FormString("group_name")

	svc := edu.GetService()
	if svc == nil {
		ctx.ServerError("GetService", nil)
		return
	}

	course, err := svc.GetCourseByID(ctx, courseID)
	if err != nil {
		ctx.ServerError("GetCourseByID", err)
		return
	}
	if course == nil {
		ctx.NotFound("Course not found", nil)
		return
	}

	if course.CreatorID != ctx.Doer.ID && !ctx.Doer.IsAdmin {
		ctx.Error(http.StatusForbidden, "You can only manage enrollments for your own courses")
		return
	}

	if username == "" {
		ctx.Flash.Error("Username is required")
		ctx.Redirect(setting.AppSubURL + "/edu/teacher/courses/" + ctx.Params(":id"))
		return
	}

	u, err := edu.GetUserByName(ctx, username)
	if err != nil {
		ctx.Flash.Error("User not found: " + err.Error())
		ctx.Redirect(setting.AppSubURL + "/edu/teacher/courses/" + ctx.Params(":id"))
		return
	}

	var r edu.RoleType
	switch role {
	case "ta":
		r = edu.RoleTA
	case "teacher":
		r = edu.RoleTeacher
	case "admin":
		r = edu.RoleAdmin
	default:
		r = edu.RoleStudent
	}

	if err := svc.EnrollUser(ctx, edu.EnrollUserOptions{CourseID: courseID, UserID: u.ID, Role: r, GroupName: groupName}); err != nil {
		if errors.Is(err, edu.ErrUserAlreadyEnrolled) {
			ctx.Flash.Error(ctx.Tr("edu.user_already_enrolled"))
			ctx.Redirect(setting.AppSubURL + "/edu/teacher/courses/" + ctx.Params(":id"))
			return
		}
		ctx.ServerError("EnrollUser", err)
		return
	}

	ctx.Flash.Success("User " + username + " enrolled")
	ctx.Redirect(setting.AppSubURL + "/edu/teacher/courses/" + ctx.Params(":id"))
}

func RemoveEnrollmentPost(ctx *context.Context) {
	courseID := ctx.ParamsInt64(":id")
	userID := ctx.FormInt64("user_id")

	svc := edu.GetService()
	if svc == nil {
		ctx.ServerError("GetService", nil)
		return
	}

	course, err := svc.GetCourseByID(ctx, courseID)
	if err != nil {
		ctx.ServerError("GetCourseByID", err)
		return
	}
	if course == nil {
		ctx.NotFound("Course not found", nil)
		return
	}

	if course.CreatorID != ctx.Doer.ID && !ctx.Doer.IsAdmin {
		ctx.Error(http.StatusForbidden, "You can only manage enrollments for your own courses")
		return
	}

	if err := svc.RemoveEnrollment(ctx, courseID, userID); err != nil {
		ctx.ServerError("RemoveEnrollment", err)
		return
	}

	ctx.Flash.Success("User removed from course")
	ctx.Redirect(setting.AppSubURL + "/edu/teacher/courses/" + ctx.Params(":id"))
}

// InitForksPost starts an asynchronous course-level init-forks operation.
// Only the course creator (or site admin) may run it; teacher role enforced by reqEduTeacher middleware.
func InitForksPost(ctx *context.Context) {
	if !isFullTeacher(ctx) {
		ctx.Error(http.StatusForbidden, "Only teachers can init forks")
		return
	}

	courseID := ctx.ParamsInt64(":id")
	svc := edu.GetService()
	if svc == nil {
		ctx.ServerError("GetService", nil)
		return
	}

	course, err := svc.GetCourseByID(ctx, courseID)
	if err != nil {
		ctx.ServerError("GetCourseByID", err)
		return
	}
	if course == nil {
		ctx.NotFound("Course not found", nil)
		return
	}
	if course.CreatorID != ctx.Doer.ID && !ctx.Doer.IsAdmin {
		ctx.Error(http.StatusForbidden, "You can only init forks for your own courses")
		return
	}

	if _, err := svc.InitCourseForks(ctx, courseID, ctx.Doer.ID); err != nil {
		switch {
		case errors.Is(err, edu.ErrTasksMasterRepoNotSet):
			ctx.Flash.Error(ctx.Tr("edu.init_forks_no_master"))
		case errors.Is(err, edu.ErrCourseHasNoOrg):
			ctx.Flash.Error(ctx.Tr("edu.init_forks_no_org"))
		default:
			ctx.ServerError("InitCourseForks", err)
			return
		}
		ctx.Redirect(setting.AppSubURL + "/edu/teacher/courses/" + ctx.Params(":id"))
		return
	}

	ctx.Flash.Success(ctx.Tr("edu.init_forks_started"))
	ctx.Redirect(setting.AppSubURL + "/edu/teacher/courses/" + ctx.Params(":id"))
}

// InitForksStatus returns JSON progress of the latest course-level init-forks task.
func InitForksStatus(ctx *context.Context) {
	courseID := ctx.ParamsInt64(":id")
	svc := edu.GetService()
	if svc == nil {
		ctx.ServerError("GetService", nil)
		return
	}

	task, err := svc.GetInitForksTaskByCourse(ctx, courseID)
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
		"total":     task.TotalUsers,
		"completed": task.Completed,
		"failed":    task.Failed,
		"error_log": task.ErrorLog,
		"updated":   task.UpdatedUnix,
	})
}
