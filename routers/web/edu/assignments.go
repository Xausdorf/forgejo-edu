package edu

import (
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"forgejo.org/internal/edu"
	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/base"
	"forgejo.org/modules/log"
	"forgejo.org/modules/setting"
	"forgejo.org/services/context"
)

const (
	tplAssignmentList        base.TplName = "edu/assignment_list"
	tplStudentAssignmentList base.TplName = "edu/student_assignment_list"
	tplAssignmentDetail      base.TplName = "edu/assignment_detail"
	tplAssignmentEdit        base.TplName = "edu/assignment_edit"
)

func StudentAssignments(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("edu.assignments")
	ctx.Data["PageIsEduAssignments"] = true

	svc := edu.GetService()

	assignments, err := svc.GetAssignmentsForUser(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetAssignmentsForUser", err)
		return
	}

	ctx.Data["Assignments"] = assignments
	ctx.Data["PageIsEduStudent"] = true
	setEduNavContext(ctx)
	ctx.HTML(http.StatusOK, tplStudentAssignmentList)
}

func TeacherAssignments(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("edu.assignments")
	ctx.Data["PageIsEduAssignments"] = true

	svc := edu.GetService()

	assignments, err := svc.GetAssignmentsForUser(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetAssignmentsForUser", err)
		return
	}

	// Build a map of courseID -> course name for display
	courseMap := make(map[int64]*edu.Course)
	for _, a := range assignments {
		if a.CourseID > 0 {
			if _, ok := courseMap[a.CourseID]; !ok {
				course, err := svc.GetCourseByID(ctx, a.CourseID)
				if err == nil && course != nil {
					courseMap[a.CourseID] = course
				}
			}
		}
	}

	ctx.Data["Assignments"] = assignments
	ctx.Data["CourseMap"] = courseMap
	setEduNavContext(ctx)
	ctx.HTML(http.StatusOK, tplAssignmentList)
}

func AssignmentDetail(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("edu.assignment_detail")
	ctx.Data["PageIsEduAssignments"] = true

	assignmentID := ctx.ParamsInt64(":id")
	svc := edu.GetService()

	assignment, err := svc.GetAssignmentByID(ctx, assignmentID)
	if err != nil {
		ctx.ServerError("GetAssignmentByID", err)
		return
	}
	if assignment == nil {
		ctx.NotFound("Assignment not found", nil)
		return
	}

	// Verify enrollment and course active status for students
	if assignment.CourseID > 0 {
		enrollment, err := edu.NewRepository().GetEnrollment(ctx, assignment.CourseID, ctx.Doer.ID)
		if err != nil {
			ctx.ServerError("GetEnrollment", err)
			return
		}

		isTeacher, _ := edu.IsTeacher(ctx, ctx.Doer.ID)
		if enrollment == nil && !isTeacher && !ctx.Doer.IsAdmin {
			ctx.NotFound("Not enrolled in this course", nil)
			return
		}

		// Block students from viewing assignments in expired courses
		if !isTeacher && !ctx.Doer.IsAdmin {
			course, err := svc.GetCourseByID(ctx, assignment.CourseID)
			if err != nil {
				ctx.ServerError("GetCourseByID", err)
				return
			}
			if course != nil && !course.IsActive() {
				ctx.Flash.Error(ctx.Tr("edu.course_expired"))
				ctx.Redirect(setting.AppSubURL + "/edu/student/assignments")
				return
			}
		}
	}

	ctx.Data["Assignment"] = assignment
	ctx.Data["DeadlinePassed"] = assignment.DeadlineUnix > 0 && time.Now().Unix() > assignment.DeadlineUnix

	repo := edu.NewRepository()
	submission, err := repo.GetSubmission(ctx, assignment.ID, ctx.Doer.ID)

	if err != nil {
		log.Error("Failed to get submission: %v", err)
	}
	ctx.Data["Submission"] = submission

	if submission != nil {
		latestResult, _ := svc.GetLatestTestResult(ctx, submission.ID)
		ctx.Data["LatestTestResult"] = latestResult

		// Resolve student fork link: org/<repoName>/src/branch/<branchName>
		if assignment.CourseID > 0 {
			course, err := svc.GetCourseByID(ctx, assignment.CourseID)
			if err == nil && course != nil && course.OrgID > 0 {
				enrollment, _ := edu.NewRepository().GetEnrollmentByCourseUser(ctx, assignment.CourseID, ctx.Doer.ID)
				if enrollment != nil && enrollment.StudentForkRepoID > 0 {
					forkRepo, _ := repo_model.GetRepositoryByID(ctx, enrollment.StudentForkRepoID)
					orgUser, _ := user_model.GetUserByID(ctx, course.OrgID)
					if forkRepo != nil && orgUser != nil {
						ctx.Data["StudentForkLink"] = orgUser.Name + "/" + forkRepo.Name + "/src/branch/" + submission.BranchName
					}
				}
			}
		}
	}

	ctx.Data["PageIsEduStudent"] = true
	setEduNavContext(ctx)
	ctx.HTML(http.StatusOK, tplAssignmentDetail)
}

// loadCoursesForAssignment populates "Courses" and "SelectedCourseID" for the
// assignment form. After Plan 3 there is no per-assignment template-repo
// dropdown — the tasks-master is fixed at the course level.
func loadCoursesForAssignment(ctx *context.Context, svc edu.EducationalService, selectedCourseID int64) {
	courses, err := svc.GetCoursesForUser(ctx, ctx.Doer.ID)
	if err != nil {
		log.Error("Failed to get courses: %v", err)
	}
	ctx.Data["Courses"] = courses
	ctx.Data["SelectedCourseID"] = selectedCourseID
}

func NewAssignment(ctx *context.Context) {
	if !isFullTeacher(ctx) {
		ctx.Error(http.StatusForbidden, "Only teachers can create assignments")
		return
	}
	ctx.Data["Title"] = "New Assignment"
	ctx.Data["PageIsEduAssignments"] = true

	svc := edu.GetService()
	selectedCourseID := ctx.FormInt64("course_id")
	loadCoursesForAssignment(ctx, svc, selectedCourseID)

	setEduNavContext(ctx)
	ctx.HTML(http.StatusOK, "edu/assignment_new")
}

func NewAssignmentPost(ctx *context.Context) {
	if !isFullTeacher(ctx) {
		ctx.Error(http.StatusForbidden, "Only teachers can create assignments")
		return
	}
	ctx.Data["Title"] = "New Assignment"
	ctx.Data["PageIsEduAssignments"] = true

	svc := edu.GetService()

	courseID := ctx.FormInt64("course_id")
	taskName := ctx.FormString("task_name")
	title := ctx.FormString("title")
	description := ctx.FormString("description")
	allowedFilesGlob := ctx.FormString("allowed_files_glob")

	// Render-with-error helper that re-populates the form with current input
	// and the courses dropdown so the user does not lose their typing.
	renderErr := func(msg template.HTML) {
		loadCoursesForAssignment(ctx, svc, courseID)
		ctx.Data["task_name"] = taskName
		ctx.Data["title"] = title
		ctx.Data["description"] = description
		ctx.Data["allowed_files_glob"] = allowedFilesGlob
		ctx.Data["deadline"] = ctx.FormString("deadline")
		ctx.RenderWithErr(msg, "edu/assignment_new", nil)
	}

	if courseID == 0 {
		renderErr(ctx.Tr("edu.course_required"))
		return
	}
	if title == "" {
		renderErr(ctx.Tr("edu.title_required"))
		return
	}
	if len(title) > 255 {
		renderErr(ctx.Tr("edu.title_too_long"))
		return
	}

	var deadlineUnix int64
	deadlineStr := ctx.FormString("deadline")
	if deadlineStr != "" {
		t, err := time.Parse("2006-01-02T15:04", deadlineStr)
		if err == nil {
			deadlineUnix = t.Unix()
		}
	}

	opts := edu.CreateAssignmentOptions{
		CourseID:         courseID,
		TaskName:         taskName,
		AllowedFilesGlob: allowedFilesGlob,
		Title:            title,
		Description:      description,
		DeadlineUnix:     deadlineUnix,
	}

	a, err := svc.CreateAssignment(ctx, opts)
	if err != nil {
		switch {
		case errors.Is(err, edu.ErrAssignmentTaskNameInvalid):
			renderErr(ctx.Tr("edu.task_name_invalid"))
		case errors.Is(err, edu.ErrAssignmentTaskNameInUse):
			renderErr(ctx.Tr("edu.task_name_in_use"))
		case errors.Is(err, edu.ErrAllowedFilesGlobRequired):
			renderErr(ctx.Tr("edu.allowed_files_required"))
		case errors.Is(err, edu.ErrTasksMasterRepoNotSet):
			renderErr(ctx.Tr("edu.distribute_no_master"))
		case errors.Is(err, edu.ErrSubmitsBranchNotFound):
			renderErr(ctx.Tr("edu.submits_branch_missing"))
		default:
			ctx.ServerError("CreateAssignment", err)
		}
		return
	}

	ctx.Redirect(setting.AppSubURL + "/edu/teacher/assignments/" + strconv.FormatInt(a.ID, 10) + "/submissions")
}

func EditAssignment(ctx *context.Context) {
	ctx.Data["Title"] = "Edit Assignment"
	ctx.Data["PageIsEduAssignments"] = true

	assignmentID := ctx.ParamsInt64(":id")
	svc := edu.GetService()

	assignment, err := svc.GetAssignmentByID(ctx, assignmentID)
	if err != nil {
		ctx.ServerError("GetAssignmentByID", err)
		return
	}
	if assignment == nil {
		ctx.NotFound("Assignment not found", nil)
		return
	}

	// Verify course ownership
	if assignment.CourseID > 0 {
		course, err := svc.GetCourseByID(ctx, assignment.CourseID)
		if err != nil {
			ctx.ServerError("GetCourseByID", err)
			return
		}
		if course != nil && course.CreatorID != ctx.Doer.ID && !ctx.Doer.IsAdmin {
			ctx.Error(http.StatusForbidden, "You can only edit assignments in your own courses")
			return
		}
	}

	ctx.Data["Assignment"] = assignment
	setEduNavContext(ctx)
	ctx.HTML(http.StatusOK, tplAssignmentEdit)
}

func EditAssignmentPost(ctx *context.Context) {
	ctx.Data["Title"] = "Edit Assignment"
	ctx.Data["PageIsEduAssignments"] = true

	assignmentID := ctx.ParamsInt64(":id")
	svc := edu.GetService()

	assignment, err := svc.GetAssignmentByID(ctx, assignmentID)
	if err != nil {
		ctx.ServerError("GetAssignmentByID", err)
		return
	}
	if assignment == nil {
		ctx.NotFound("Assignment not found", nil)
		return
	}

	// Verify course ownership
	if assignment.CourseID > 0 {
		course, err := svc.GetCourseByID(ctx, assignment.CourseID)
		if err != nil {
			ctx.ServerError("GetCourseByID", err)
			return
		}
		if course != nil && course.CreatorID != ctx.Doer.ID && !ctx.Doer.IsAdmin {
			ctx.Error(http.StatusForbidden, "You can only edit assignments in your own courses")
			return
		}
	}

	assignment.Title = ctx.FormString("title")
	assignment.Description = ctx.FormString("description")
	assignment.AllowedFilesGlob = ctx.FormString("allowed_files_glob")

	if assignment.Title == "" {
		ctx.Data["Assignment"] = assignment
		ctx.RenderWithErr("Title is required.", tplAssignmentEdit, nil)
		return
	}
	if len(assignment.Title) > 255 {
		ctx.Data["Assignment"] = assignment
		ctx.RenderWithErr(ctx.Tr("edu.title_too_long"), tplAssignmentEdit, nil)
		return
	}
	if assignment.AllowedFilesGlob == "" {
		ctx.Data["Assignment"] = assignment
		ctx.RenderWithErr(ctx.Tr("edu.allowed_files_required"), tplAssignmentEdit, nil)
		return
	}

	deadlineStr := ctx.FormString("deadline")
	if deadlineStr != "" {
		t, err := time.Parse("2006-01-02T15:04", deadlineStr)
		if err == nil {
			assignment.DeadlineUnix = t.Unix()
		}
	} else {
		assignment.DeadlineUnix = 0
	}

	if err := svc.UpdateAssignment(ctx, assignment); err != nil {
		ctx.ServerError("UpdateAssignment", err)
		return
	}

	ctx.Redirect(setting.AppSubURL + "/edu/teacher/assignments/" + ctx.Params(":id") + "/submissions")
}

func DeleteAssignmentPost(ctx *context.Context) {
	assignmentID := ctx.ParamsInt64(":id")
	svc := edu.GetService()

	assignment, err := svc.GetAssignmentByID(ctx, assignmentID)
	if err != nil {
		ctx.ServerError("GetAssignmentByID", err)
		return
	}
	if assignment == nil {
		ctx.NotFound("Assignment not found", nil)
		return
	}

	// Verify course ownership
	if assignment.CourseID > 0 {
		course, err := svc.GetCourseByID(ctx, assignment.CourseID)
		if err != nil {
			ctx.ServerError("GetCourseByID", err)
			return
		}
		if course != nil && course.CreatorID != ctx.Doer.ID && !ctx.Doer.IsAdmin {
			ctx.Error(http.StatusForbidden, "You can only delete assignments in your own courses")
			return
		}
	}

	if err := svc.DeleteAssignment(ctx, assignmentID); err != nil {
		ctx.ServerError("DeleteAssignment", err)
		return
	}

	ctx.Redirect(setting.AppSubURL + "/edu/teacher/assignments")
}

// DistributePost starts an asynchronous bulk-push of submits/<task_name> from
// the course's tasks-master into every student fork. Only the course creator
// (or site admin) may run it; full-teacher role enforced by reqEduTeacher.
func DistributePost(ctx *context.Context) {
	if !isFullTeacher(ctx) {
		ctx.Error(http.StatusForbidden, "Only teachers can distribute assignments")
		return
	}

	assignmentID := ctx.ParamsInt64(":id")
	svc := edu.GetService()
	if svc == nil {
		ctx.ServerError("GetService", nil)
		return
	}

	assignment, err := svc.GetAssignmentByID(ctx, assignmentID)
	if err != nil {
		ctx.ServerError("GetAssignmentByID", err)
		return
	}
	if assignment == nil {
		ctx.NotFound("Assignment not found", nil)
		return
	}

	course, err := svc.GetCourseByID(ctx, assignment.CourseID)
	if err != nil {
		ctx.ServerError("GetCourseByID", err)
		return
	}
	if course == nil {
		ctx.NotFound("Course not found", nil)
		return
	}
	if course.CreatorID != ctx.Doer.ID && !ctx.Doer.IsAdmin {
		ctx.Error(http.StatusForbidden, "You can only distribute assignments in your own courses")
		return
	}

	if _, err := svc.DistributeAssignment(ctx, assignmentID, ctx.Doer.ID); err != nil {
		switch {
		case errors.Is(err, edu.ErrTasksMasterRepoNotSet):
			ctx.Flash.Error(ctx.Tr("edu.distribute_no_master"))
		case errors.Is(err, edu.ErrInitForksNotDone):
			ctx.Flash.Error(ctx.Tr("edu.distribute_no_init_forks"))
		default:
			ctx.ServerError("DistributeAssignment", err)
			return
		}
		ctx.Redirect(setting.AppSubURL + "/edu/teacher/assignments/" + ctx.Params(":id") + "/submissions")
		return
	}

	ctx.Flash.Success(ctx.Tr("edu.distribute_started"))
	ctx.Redirect(setting.AppSubURL + "/edu/teacher/assignments/" + ctx.Params(":id") + "/submissions")
}

// DistributeStatus returns JSON progress of the latest distribute task for an assignment.
func DistributeStatus(ctx *context.Context) {
	assignmentID := ctx.ParamsInt64(":id")
	svc := edu.GetService()
	if svc == nil {
		ctx.ServerError("GetService", nil)
		return
	}

	task, err := svc.GetDistributeTaskByAssignment(ctx, assignmentID)
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
		"total":     task.TotalEnrollments,
		"pushed":    task.Pushed,
		"failed":    task.Failed,
		"error_log": task.ErrorLog,
		"updated":   task.UpdatedUnix,
	})
}
