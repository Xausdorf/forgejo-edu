package edu

import (
	"net/http"
	"strings"

	"forgejo.org/internal/edu"
	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/base"
	"forgejo.org/services/context"
)

const (
	tplInstructorSubmissions base.TplName = "edu/instructor_submissions"
)

func InstructorSubmissions(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("edu.instructor_panel")
	ctx.Data["PageIsEduAssignments"] = true

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

	if !isEduInstructor(ctx) {
		ctx.Error(http.StatusForbidden, "Only instructors can view this page")
		return
	}

	ctx.Data["Assignment"] = assignment

	course, err := svc.GetCourseByID(ctx, assignment.CourseID)
	if err != nil {
		ctx.ServerError("GetCourseByID", err)
		return
	}
	ctx.Data["Course"] = course

	submissions, err := svc.GetSubmissions(ctx, assignmentID)
	if err != nil {
		ctx.ServerError("GetSubmissions", err)
		return
	}

	// Load enrollments for the course so we can map submission → group/fork
	enrollments, err := svc.GetEnrollments(ctx, assignment.CourseID)
	if err != nil {
		ctx.ServerError("GetEnrollments", err)
		return
	}
	enrollmentByID := make(map[int64]*edu.CourseEnrollment, len(enrollments))
	enrollmentByUser := make(map[int64]*edu.CourseEnrollment, len(enrollments))
	groupSet := make(map[string]struct{})
	for _, e := range enrollments {
		enrollmentByID[e.ID] = e
		enrollmentByUser[e.UserID] = e
		if e.GroupName != "" {
			groupSet[e.GroupName] = struct{}{}
		}
	}
	groups := make([]string, 0, len(groupSet))
	for g := range groupSet {
		groups = append(groups, g)
	}

	// Parse ?group= filter (multi-select supported via repeated ?group=g1&group=g2 or comma-separated)
	rawGroups := ctx.FormStrings("group")
	selectedGroups := make(map[string]struct{})
	for _, raw := range rawGroups {
		for _, g := range strings.Split(raw, ",") {
			g = strings.TrimSpace(g)
			if g != "" {
				selectedGroups[g] = struct{}{}
			}
		}
	}

	// Filter submissions by group (if any selected) and load user / repo data
	filteredSubs := make([]*edu.Submission, 0, len(submissions))
	userIDs := make([]int64, 0, len(submissions))
	repoIDs := make([]int64, 0, len(submissions))
	for _, s := range submissions {
		enr := enrollmentByID[s.EnrollmentID]
		if enr == nil {
			enr = enrollmentByUser[s.UserID]
		}
		if len(selectedGroups) > 0 {
			if enr == nil {
				continue
			}
			if _, ok := selectedGroups[enr.GroupName]; !ok {
				continue
			}
		}
		filteredSubs = append(filteredSubs, s)
		userIDs = append(userIDs, s.UserID)
		if enr != nil && enr.StudentForkRepoID > 0 {
			repoIDs = append(repoIDs, enr.StudentForkRepoID)
		}
	}

	users, err := user_model.GetUsersByIDs(ctx, userIDs)
	if err != nil {
		ctx.ServerError("GetUsersByIDs", err)
		return
	}
	userMap := make(map[int64]*user_model.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}

	repoMap := make(map[int64]*repo_model.Repository)
	for _, repoID := range repoIDs {
		r, err := repo_model.GetRepositoryByID(ctx, repoID)
		if err == nil && r != nil {
			repoMap[repoID] = r
		}
	}

	repoLinkMap := make(map[int64]string, len(filteredSubs))
	enrollmentForSub := make(map[int64]*edu.CourseEnrollment, len(filteredSubs))
	if course != nil && course.OrgID > 0 {
		orgUser, _ := user_model.GetUserByID(ctx, course.OrgID)
		for _, s := range filteredSubs {
			enr := enrollmentByID[s.EnrollmentID]
			if enr == nil {
				enr = enrollmentByUser[s.UserID]
			}
			enrollmentForSub[s.ID] = enr
			if enr != nil && enr.StudentForkRepoID > 0 && orgUser != nil {
				if r, ok := repoMap[enr.StudentForkRepoID]; ok && r != nil {
					repoLinkMap[s.ID] = orgUser.Name + "/" + r.Name + "/src/branch/" + s.BranchName
				}
			}
		}
	}

	testResultMap := make(map[int64]*edu.TestResult, len(filteredSubs))
	for _, sub := range filteredSubs {
		tr, _ := svc.GetLatestTestResult(ctx, sub.ID)
		if tr != nil {
			testResultMap[sub.ID] = tr
		}
	}

	distributeTask, err := svc.GetDistributeTaskByAssignment(ctx, assignmentID)
	if err != nil {
		ctx.ServerError("GetDistributeTaskByAssignment", err)
		return
	}

	ctx.Data["Submissions"] = filteredSubs
	ctx.Data["UserMap"] = userMap
	ctx.Data["EnrollmentMap"] = enrollmentForSub
	ctx.Data["RepoLinkMap"] = repoLinkMap
	ctx.Data["TestResultMap"] = testResultMap
	ctx.Data["DistributeTask"] = distributeTask
	ctx.Data["AvailableGroups"] = groups
	ctx.Data["SelectedGroups"] = rawGroups

	setEduNavContext(ctx)
	ctx.HTML(http.StatusOK, tplInstructorSubmissions)
}
