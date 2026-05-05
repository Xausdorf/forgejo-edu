package edu

import (
	"net/http"
	"strings"

	"forgejo.org/internal/edu"
	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/base"
	"forgejo.org/modules/log"
	"forgejo.org/modules/setting"
	"forgejo.org/services/context"
)

const (
	tplSubmissionReview base.TplName = "edu/submission_review"
)

// reviewURL returns the canonical URL for the review page of a given
// (assignment, submission) pair, used by handlers that redirect back after
// a POST.
func reviewURL(ctx *context.Context) string {
	return setting.AppSubURL + "/edu/teacher/assignments/" + ctx.Params(":id") +
		"/submissions/" + ctx.Params(":subID")
}

func submissionsURL(ctx *context.Context) string {
	return setting.AppSubURL + "/edu/teacher/assignments/" + ctx.Params(":id") + "/submissions"
}

// loadReviewSubject is shared by all review handlers. Returns (assignment, submission, ok).
// If ok is false, the response has already been written (404/forbidden/server-error).
func loadReviewSubject(ctx *context.Context) (*edu.Assignment, *edu.Submission, bool) {
	if !isEduInstructor(ctx) {
		ctx.Error(http.StatusForbidden, "Only instructors can access this page")
		return nil, nil, false
	}
	assignmentID := ctx.ParamsInt64(":id")
	subID := ctx.ParamsInt64(":subID")
	svc := edu.GetService()
	if svc == nil {
		ctx.ServerError("GetService", nil)
		return nil, nil, false
	}
	assignment, err := svc.GetAssignmentByID(ctx, assignmentID)
	if err != nil {
		ctx.ServerError("GetAssignmentByID", err)
		return nil, nil, false
	}
	if assignment == nil {
		ctx.NotFound("Assignment not found", nil)
		return nil, nil, false
	}
	submission, err := svc.GetSubmissionByID(ctx, subID)
	if err != nil {
		ctx.ServerError("GetSubmissionByID", err)
		return nil, nil, false
	}
	if submission == nil || submission.AssignmentID != assignment.ID {
		ctx.NotFound("Submission not found for this assignment", nil)
		return nil, nil, false
	}
	return assignment, submission, true
}

// SubmissionReview renders the review page for a single submission.
func SubmissionReview(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("edu.review.title")
	ctx.Data["PageIsEduAssignments"] = true

	assignment, submission, ok := loadReviewSubject(ctx)
	if !ok {
		return
	}
	ctx.Data["Assignment"] = assignment
	ctx.Data["Submission"] = submission

	svc := edu.GetService()

	if u, err := user_model.GetUserByID(ctx, submission.UserID); err == nil {
		ctx.Data["Student"] = u
	} else {
		log.Error("review: load student %d: %v", submission.UserID, err)
	}

	if submission.GradedByID > 0 {
		if g, err := user_model.GetUserByID(ctx, submission.GradedByID); err == nil {
			ctx.Data["Grader"] = g
		}
	}

	if assignment.CourseID > 0 {
		if course, err := svc.GetCourseByID(ctx, assignment.CourseID); err == nil && course != nil {
			ctx.Data["Course"] = course
			repo := edu.NewRepository()
			enrollment, _ := repo.GetEnrollmentByCourseUser(ctx, assignment.CourseID, submission.UserID)
			if enrollment != nil && enrollment.StudentForkRepoID > 0 && course.OrgID > 0 {
				forkRepo, _ := repo_model.GetRepositoryByID(ctx, enrollment.StudentForkRepoID)
				orgUser, _ := user_model.GetUserByID(ctx, course.OrgID)
				if forkRepo != nil && orgUser != nil {
					ctx.Data["ForkLink"] = orgUser.Name + "/" + forkRepo.Name + "/src/branch/" + submission.BranchName
				}
			}
		}
	}

	if results, err := svc.GetTestResults(ctx, submission.ID); err == nil {
		ctx.Data["TestResults"] = results
	}

	if submission.PullRequestID != 0 {
		pr, err := edu.NewForgejoAdapter().GetPullRequest(ctx, submission.PullRequestID)
		if err != nil {
			log.Error("review: load PR %d: %v", submission.PullRequestID, err)
		} else if pr != nil {
			ctx.Data["PullRequest"] = pr
			if pr.BaseRepo != nil && pr.Issue != nil {
				ctx.Data["PullRequestURL"] = setting.AppSubURL + "/" +
					pr.BaseRepo.OwnerName + "/" + pr.BaseRepo.Name +
					"/pulls/" + intToString(pr.Issue.Index)
			}
		}
		if files, err := edu.NewForgejoAdapter().GetPullRequestChangedFiles(ctx, submission.PullRequestID); err == nil {
			ctx.Data["ChangedFiles"] = files
		} else {
			log.Error("review: changed files for PR %d: %v", submission.PullRequestID, err)
		}
		if comments, err := edu.NewForgejoAdapter().GetPullRequestComments(ctx, submission.PullRequestID); err == nil {
			ctx.Data["Comments"] = comments
		} else {
			log.Error("review: comments for PR %d: %v", submission.PullRequestID, err)
		}
	}

	setEduNavContext(ctx)
	ctx.HTML(http.StatusOK, tplSubmissionReview)
}

// ApproveSubmissionPost records the TA's approval (grade + comment), flips
// status to approved.
func ApproveSubmissionPost(ctx *context.Context) {
	_, _, ok := loadReviewSubject(ctx)
	if !ok {
		return
	}
	subID := ctx.ParamsInt64(":subID")
	grade := int(ctx.FormInt64("grade"))
	comment := ctx.FormString("comment")

	if grade < 0 || grade > 100 {
		ctx.Flash.Error(ctx.Tr("edu.review.grade") + ": 0..100")
		ctx.Redirect(reviewURL(ctx))
		return
	}
	if len(comment) > 10000 {
		ctx.Flash.Error(ctx.Tr("edu.comment_too_long"))
		ctx.Redirect(reviewURL(ctx))
		return
	}

	if err := edu.GetService().ApproveSubmission(ctx, subID, grade, comment, ctx.Doer.ID); err != nil {
		ctx.ServerError("ApproveSubmission", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("edu.review.approve_success"))
	ctx.Redirect(reviewURL(ctx))
}

// MergeSubmissionPost merges the submission's PR via eduadmin and flips
// status to merged. Pre-conditions are enforced by the service layer.
func MergeSubmissionPost(ctx *context.Context) {
	_, submission, ok := loadReviewSubject(ctx)
	if !ok {
		return
	}
	if submission.PullRequestID == 0 {
		ctx.Flash.Error(ctx.Tr("edu.review.no_pr_to_merge"))
		ctx.Redirect(reviewURL(ctx))
		return
	}
	if submission.Status != edu.StatusSubmissionApproved {
		ctx.Flash.Error(ctx.Tr("edu.review.must_be_approved_to_merge"))
		ctx.Redirect(reviewURL(ctx))
		return
	}
	if err := edu.GetService().MergeSubmission(ctx, submission.ID); err != nil {
		log.Error("merge submission %d: %v", submission.ID, err)
		ctx.Flash.Error(err.Error())
		ctx.Redirect(reviewURL(ctx))
		return
	}
	ctx.Flash.Success(ctx.Tr("edu.review.merge_success"))
	ctx.Redirect(submissionsURL(ctx))
}

// CommentSubmissionPost adds a TA comment in the PR issue thread on behalf
// of the calling user.
func CommentSubmissionPost(ctx *context.Context) {
	_, submission, ok := loadReviewSubject(ctx)
	if !ok {
		return
	}
	if submission.PullRequestID == 0 {
		ctx.Flash.Error(ctx.Tr("edu.review.no_pr_to_comment"))
		ctx.Redirect(reviewURL(ctx))
		return
	}
	body := strings.TrimSpace(ctx.FormString("body"))
	if body == "" {
		ctx.Flash.Error(ctx.Tr("edu.review.empty_comment"))
		ctx.Redirect(reviewURL(ctx))
		return
	}
	if err := edu.GetService().AddSubmissionComment(ctx, submission.ID, body, ctx.Doer); err != nil {
		log.Error("add submission comment %d: %v", submission.ID, err)
		ctx.Flash.Error(err.Error())
		ctx.Redirect(reviewURL(ctx))
		return
	}
	ctx.Flash.Success(ctx.Tr("edu.review.comment_added"))
	ctx.Redirect(reviewURL(ctx))
}

// ResetApprovalPost reverts status from approved back to done.
func ResetApprovalPost(ctx *context.Context) {
	_, submission, ok := loadReviewSubject(ctx)
	if !ok {
		return
	}
	if submission.Status != edu.StatusSubmissionApproved {
		ctx.Flash.Error(ctx.Tr("edu.review.must_be_approved_to_reset"))
		ctx.Redirect(reviewURL(ctx))
		return
	}
	if err := edu.GetService().ResetApproval(ctx, submission.ID); err != nil {
		ctx.ServerError("ResetApproval", err)
		return
	}
	ctx.Flash.Success(ctx.Tr("edu.review.reset_approval_success"))
	ctx.Redirect(reviewURL(ctx))
}

// intToString avoids importing strconv just for one int64 conversion.
func intToString(i int64) string {
	if i == 0 {
		return "0"
	}
	negative := i < 0
	if negative {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if negative {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
