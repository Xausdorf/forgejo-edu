package edu

import (
	"context"
	"fmt"
	"strings"

	actions_model "forgejo.org/models/actions"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/git"
	"forgejo.org/modules/log"
	"forgejo.org/services/notify"
)

// systemDoerName is the Forgejo account the edu server acts as for PR creation,
// system comments, and merges (configured in edu-docker/docker-compose.yml).
const systemDoerName = "eduadmin"

type EduNotifier struct {
	notify.NullNotifier
	repo  Repository
	pulls PullRequestService
	logs  ActionLogReader
	users UserCreator
}

var _ notify.Notifier = &EduNotifier{}

func RegisterNotifier(repo Repository, pulls PullRequestService, logs ActionLogReader, users UserCreator) {
	notify.RegisterNotifier(&EduNotifier{
		repo:  repo,
		pulls: pulls,
		logs:  logs,
		users: users,
	})
}

func (n *EduNotifier) ActionRunNowDone(
	ctx context.Context,
	run *actions_model.ActionRun,
	priorStatus actions_model.Status,
	lastRun *actions_model.ActionRun,
) {
	if run == nil || run.RepoID == 0 {
		return
	}
	ref := git.RefName(run.Ref)
	if !ref.IsBranch() {
		return
	}
	branch := ref.BranchName()
	if !strings.HasPrefix(branch, "submits/") {
		return
	}
	if err := n.processCompletedRun(ctx, run, branch); err != nil {
		log.Error("edu notifier: process run %d (repo %d, branch %s): %v",
			run.ID, run.RepoID, branch, err)
	}
}

// processCompletedRun is the testable body of ActionRunNowDone. Returns an
// error only for genuinely unexpected failures (DB errors etc.); routine
// "no edu submission for this run" cases return nil silently.
func (n *EduNotifier) processCompletedRun(ctx context.Context, run *actions_model.ActionRun, branch string) error {
	enrollment, err := n.repo.GetEnrollmentByForkRepo(ctx, run.RepoID)
	if err != nil {
		return fmt.Errorf("get enrollment: %w", err)
	}
	if enrollment == nil {
		return nil // not an edu fork
	}
	taskName := strings.TrimPrefix(branch, "submits/")
	if taskName == "" {
		return nil
	}
	assignment, err := n.repo.GetAssignmentByCourseAndTask(ctx, enrollment.CourseID, taskName)
	if err != nil {
		return fmt.Errorf("get assignment: %w", err)
	}
	if assignment == nil {
		return nil // branch not bound to any assignment
	}
	submission, err := n.repo.GetSubmissionByEnrollmentAssignment(ctx, enrollment.ID, assignment.ID)
	if err != nil {
		return fmt.Errorf("get submission: %w", err)
	}
	if submission == nil {
		// Distribute has not been run yet — student pushed before bulk-distribute
		// created the row. Skip silently; next push after distribute will be
		// handled normally.
		return nil
	}

	doer, err := n.users.GetUserByName(ctx, systemDoerName)
	if err != nil || doer == nil {
		return fmt.Errorf("system doer %q not found: %w", systemDoerName, err)
	}

	course, err := n.repo.GetCourseByID(ctx, enrollment.CourseID)
	if err != nil {
		return fmt.Errorf("get course: %w", err)
	}
	if course == nil {
		return fmt.Errorf("course %d not found", enrollment.CourseID)
	}

	if submission.PullRequestID == 0 {
		prID, prErr := n.ensurePullRequest(ctx, run.RepoID, branch, assignment, course, enrollment, doer)
		if prErr != nil {
			// PR creation failure is non-fatal for grading flow — we still
			// want to record the TestResult and constraint result. Just log
			// and proceed without prID.
			log.Error("edu notifier: create PR for submission %d: %v", submission.ID, prErr)
		} else if prID != 0 {
			submission.PullRequestID = prID
			if err := n.repo.UpdateSubmissionPullRequestID(ctx, submission.ID, prID); err != nil {
				log.Error("edu notifier: store PR id %d on submission %d: %v", prID, submission.ID, err)
			}
		}
	}

	baseBranch, err := n.resolveBaseBranch(ctx, run.RepoID)
	if err != nil {
		return fmt.Errorf("resolve base branch: %w", err)
	}
	changedFiles, err := n.pulls.GetBranchChangedFiles(ctx, run.RepoID, branch, baseBranch)
	if err != nil {
		return fmt.Errorf("changed files: %w", err)
	}
	violations := MatchAllowedFilesGlob(assignment.AllowedFilesGlob, changedFiles)
	if len(violations) > 0 {
		if submission.PullRequestID != 0 {
			body := buildConstraintViolationComment(assignment.TaskName, assignment.AllowedFilesGlob, violations)
			if _, err := n.pulls.AddPullRequestComment(ctx, submission.PullRequestID, body, doer); err != nil {
				log.Error("edu notifier: add constraint comment to PR %d: %v", submission.PullRequestID, err)
			}
		}
		if err := n.recordTestResult(ctx, submission.ID, run.CommitSHA, 0, "constraint violation: "+strings.Join(violations, ", ")); err != nil {
			log.Error("edu notifier: record constraint TestResult for submission %d: %v", submission.ID, err)
		}
		return n.setSubmissionStatus(ctx, submission, StatusSubmissionFailed)
	}

	logLines, err := n.logs.ReadRunLogLines(ctx, run.ID)
	if err != nil {
		log.Warn("edu notifier: read logs for run %d: %v", run.ID, err)
		logLines = nil
	}
	grade, gradeFound := ParseGradeFromLogLines(logLines)

	score := 0
	details := "ci done"
	if gradeFound {
		score = grade
		details = fmt.Sprintf("ci done; ::edu-grade::%d", grade)
	}
	if err := n.recordTestResult(ctx, submission.ID, run.CommitSHA, score, details); err != nil {
		log.Error("edu notifier: record TestResult for submission %d: %v", submission.ID, err)
	}

	if !run.Status.IsSuccess() {
		return n.setSubmissionStatus(ctx, submission, StatusSubmissionFailed)
	}
	if gradeFound && !submission.ManualGrade {
		if err := n.repo.AutoGradeSubmission(ctx, submission.ID, grade); err != nil {
			return fmt.Errorf("auto-grade submission %d: %w", submission.ID, err)
		}
		// AutoGradeSubmission already sets status=done.
		return nil
	}
	return n.setSubmissionStatus(ctx, submission, StatusSubmissionDone)
}

// ensurePullRequest creates the intra-repo PR submits/<task> -> main and returns its ID.
// The PR title is prefixed with [enrollment.GroupName] when the student belongs to a stream group.
func (n *EduNotifier) ensurePullRequest(
	ctx context.Context,
	repoID int64,
	branch string,
	assignment *Assignment,
	course *Course,
	enrollment *CourseEnrollment,
	doer *user_model.User,
) (int64, error) {
	baseBranch, err := n.resolveBaseBranch(ctx, repoID)
	if err != nil {
		return 0, fmt.Errorf("resolve base branch: %w", err)
	}
	pr, err := n.pulls.CreatePullRequest(ctx, CreatePullRequestOptions{
		BaseRepoID: repoID,
		BaseBranch: baseBranch,
		HeadRepoID: repoID,
		HeadBranch: branch,
		Title:      formatSubmissionPRTitle(assignment.TaskName, enrollment.GroupName),
		Body:       fmt.Sprintf("Auto-submitted by Edu CI for task `%s` in course **%s**.", assignment.TaskName, course.Name),
		Doer:       doer,
	})
	if err != nil {
		return 0, err
	}
	if pr == nil {
		return 0, fmt.Errorf("create PR returned nil")
	}
	return pr.ID, nil
}

// resolveBaseBranch returns the default branch of the student fork. Hardcoded:
// student forks are initialised with main as default and the GitLab-model design
// uses literal "main" everywhere.
func (n *EduNotifier) resolveBaseBranch(ctx context.Context, repoID int64) (string, error) {
	_ = ctx
	_ = repoID
	return "main", nil
}

func (n *EduNotifier) setSubmissionStatus(ctx context.Context, sub *Submission, status SubmissionStatus) error {
	sub.Status = status
	if err := n.repo.UpdateSubmission(ctx, sub); err != nil {
		return fmt.Errorf("update submission status: %w", err)
	}
	return nil
}

func (n *EduNotifier) recordTestResult(ctx context.Context, submissionID int64, commitSHA string, score int, details string) error {
	tr := &TestResult{
		SubmissionID: submissionID,
		CommitSHA:    commitSHA,
		Score:        score,
		Details:      details,
	}
	return n.repo.CreateTestResult(ctx, tr)
}

// buildConstraintViolationComment is the system PR comment body posted when
// changed files don't match Assignment.AllowedFilesGlob.
func buildConstraintViolationComment(taskName, glob string, violations []string) string {
	var b strings.Builder
	b.WriteString("❌ **Constraint check failed** — the following files are not allowed for task `")
	b.WriteString(taskName)
	b.WriteString("`:\n\n")
	for _, f := range violations {
		b.WriteString("- `")
		b.WriteString(f)
		b.WriteString("`\n")
	}
	b.WriteString("\nAllowed pattern: `")
	b.WriteString(glob)
	b.WriteString("`. Please remove the unrelated files and push again.")
	return b.String()
}

// formatSubmissionPRTitle builds the PR title for an auto-submitted task.
// When the enrollment has a non-empty GroupName, it is included as a bracketed
// prefix so instructors can scan submissions by stream group at a glance.
func formatSubmissionPRTitle(taskName, groupName string) string {
	if groupName == "" {
		return "Submit: " + taskName
	}
	return "[" + groupName + "] Submit: " + taskName
}
