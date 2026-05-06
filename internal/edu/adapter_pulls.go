package edu

import (
	"context"
	"fmt"
	"strings"

	"forgejo.org/models"
	issues_model "forgejo.org/models/issues"
	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/git"
	issue_service "forgejo.org/services/issue"
	pull_service "forgejo.org/services/pull"
)

// CreatePullRequestOptions describes a PR creation request.
type CreatePullRequestOptions struct {
	BaseRepoID int64
	BaseBranch string
	HeadRepoID int64
	HeadBranch string
	Title      string
	Body       string
	Doer       *user_model.User
}

// CreatePullRequest creates a PR HeadRepo:HeadBranch -> BaseRepo:BaseBranch.
func (a *ForgejoAdapter) CreatePullRequest(ctx context.Context, opts CreatePullRequestOptions) (*issues_model.PullRequest, error) {
	baseRepo, err := repo_model.GetRepositoryByID(ctx, opts.BaseRepoID)
	if err != nil {
		return nil, fmt.Errorf("load base repo: %w", err)
	}
	headRepo, err := repo_model.GetRepositoryByID(ctx, opts.HeadRepoID)
	if err != nil {
		return nil, fmt.Errorf("load head repo: %w", err)
	}

	pr := &issues_model.PullRequest{
		HeadRepoID: headRepo.ID,
		BaseRepoID: baseRepo.ID,
		HeadBranch: opts.HeadBranch,
		BaseBranch: opts.BaseBranch,
		HeadRepo:   headRepo,
		BaseRepo:   baseRepo,
		Type:       issues_model.PullRequestGitea,
	}
	prIssue := &issues_model.Issue{
		RepoID:   baseRepo.ID,
		Title:    opts.Title,
		PosterID: opts.Doer.ID,
		Poster:   opts.Doer,
		IsPull:   true,
		Content:  opts.Body,
	}

	// NewPullRequest(ctx, repo, issue, labelIDs, uuids, pr, assigneeIDs)
	if err := pull_service.NewPullRequest(ctx, baseRepo, prIssue, nil, nil, pr, nil); err != nil {
		return nil, fmt.Errorf("create PR: %w", err)
	}
	return pr, nil
}

// MergePullRequestOptions describes a PR merge request.
type MergePullRequestOptions struct {
	PullRequestID int64
	Doer          *user_model.User
	MergeStyle    string // "merge", "rebase", "squash"
	Message       string
}

// MergePullRequest merges the given pull request.
func (a *ForgejoAdapter) MergePullRequest(ctx context.Context, opts MergePullRequestOptions) error {
	pr, err := issues_model.GetPullRequestByID(ctx, opts.PullRequestID)
	if err != nil {
		return fmt.Errorf("load PR: %w", err)
	}
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return err
	}
	if err := pr.LoadHeadRepo(ctx); err != nil {
		return err
	}

	gitRepo, err := git.OpenRepository(ctx, pr.BaseRepo.RepoPath())
	if err != nil {
		return err
	}
	defer gitRepo.Close()

	style := repo_model.MergeStyle(opts.MergeStyle)
	return pull_service.Merge(ctx, pr, opts.Doer, gitRepo, style, "", opts.Message, false)
}

// AddPullRequestComment adds a plain comment to the given pull request.
func (a *ForgejoAdapter) AddPullRequestComment(ctx context.Context, prID int64, body string, doer *user_model.User) (*issues_model.Comment, error) {
	pr, err := issues_model.GetPullRequestByID(ctx, prID)
	if err != nil {
		return nil, err
	}
	if err := pr.LoadIssue(ctx); err != nil {
		return nil, err
	}
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return nil, err
	}
	return issue_service.CreateIssueComment(ctx, doer, pr.BaseRepo, pr.Issue, body, nil)
}

// GetPullRequestChangedFiles returns the list of files changed in the given pull request.
func (a *ForgejoAdapter) GetPullRequestChangedFiles(ctx context.Context, prID int64) ([]string, error) {
	pr, err := issues_model.GetPullRequestByID(ctx, prID)
	if err != nil {
		return nil, err
	}
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return nil, err
	}

	gitRepo, err := git.OpenRepository(ctx, pr.BaseRepo.RepoPath())
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	mergeBase, _, err := gitRepo.GetMergeBase("", pr.BaseBranch, pr.HeadBranch)
	if err != nil {
		return nil, err
	}
	headCommit, err := gitRepo.GetBranchCommit(pr.HeadBranch)
	if err != nil {
		return nil, err
	}

	stdout, _, err := git.NewCommand(ctx, "diff", "--name-only").
		AddDynamicArguments(mergeBase+"..."+headCommit.ID.String()).
		RunStdString(&git.RunOpts{Dir: pr.BaseRepo.RepoPath()})
	if err != nil {
		return nil, err
	}

	raw := strings.TrimSpace(stdout)
	if raw == "" {
		return []string{}, nil
	}
	return strings.Split(raw, "\n"), nil
}

// GetPullRequestComments returns all plain comments on the given pull request.
func (a *ForgejoAdapter) GetPullRequestComments(ctx context.Context, prID int64) ([]*issues_model.Comment, error) {
	pr, err := issues_model.GetPullRequestByID(ctx, prID)
	if err != nil {
		return nil, err
	}
	if err := pr.LoadIssue(ctx); err != nil {
		return nil, err
	}
	return issues_model.FindComments(ctx, &issues_model.FindCommentsOptions{
		IssueID: pr.IssueID,
		Type:    issues_model.CommentTypeComment,
	})
}

// GetPullRequest loads a PR by ID with HeadRepo, BaseRepo, Issue and Poster
// preloaded — enough to render a link, title, status, head/base branches in
// the edu submission-review template.
func (a *ForgejoAdapter) GetPullRequest(ctx context.Context, prID int64) (*issues_model.PullRequest, error) {
	pr, err := issues_model.GetPullRequestByID(ctx, prID)
	if err != nil {
		return nil, fmt.Errorf("load PR: %w", err)
	}
	if err := pr.LoadHeadRepo(ctx); err != nil {
		return nil, fmt.Errorf("load head repo: %w", err)
	}
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return nil, fmt.Errorf("load base repo: %w", err)
	}
	if err := pr.LoadIssue(ctx); err != nil {
		return nil, fmt.Errorf("load issue: %w", err)
	}
	if pr.Issue != nil && pr.Issue.PosterID > 0 {
		if err := pr.Issue.LoadPoster(ctx); err != nil {
			return nil, fmt.Errorf("load poster: %w", err)
		}
	}
	return pr, nil
}

// Returns (nil, nil) when no open PR matches — callers branch on that.
func (a *ForgejoAdapter) GetUnmergedPullRequest(ctx context.Context, headRepoID, baseRepoID int64, headBranch, baseBranch string) (*issues_model.PullRequest, error) {
	pr, err := issues_model.GetUnmergedPullRequest(ctx, headRepoID, baseRepoID, headBranch, baseBranch, issues_model.PullRequestFlowGithub)
	if err != nil {
		if issues_model.IsErrPullRequestNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get unmerged pr: %w", err)
	}
	if err := pr.LoadIssue(ctx); err != nil {
		return nil, fmt.Errorf("load issue: %w", err)
	}
	return pr, nil
}

// Wrapped so the service layer does not import forgejo.org/models directly.
func (a *ForgejoAdapter) IsMergeConflictError(err error) bool {
	return err != nil && models.IsErrMergeConflicts(err)
}

// GetBranchChangedFiles returns files changed in branch relative to baseBranch
// in the same repository (git diff --name-only baseBranch...branch).
func (a *ForgejoAdapter) GetBranchChangedFiles(ctx context.Context, repoID int64, branch, baseBranch string) ([]string, error) {
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("load repo: %w", err)
	}
	gitRepo, err := git.OpenRepository(ctx, repo.RepoPath())
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}
	defer gitRepo.Close()

	mergeBase, _, err := gitRepo.GetMergeBase("", baseBranch, branch)
	if err != nil {
		return nil, fmt.Errorf("merge base %s..%s: %w", baseBranch, branch, err)
	}
	headCommit, err := gitRepo.GetBranchCommit(branch)
	if err != nil {
		return nil, fmt.Errorf("get branch commit %s: %w", branch, err)
	}

	stdout, _, err := git.NewCommand(ctx, "diff", "--name-only").
		AddDynamicArguments(mergeBase + "..." + headCommit.ID.String()).
		RunStdString(&git.RunOpts{Dir: repo.RepoPath()})
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	raw := strings.TrimSpace(stdout)
	if raw == "" {
		return []string{}, nil
	}
	return strings.Split(raw, "\n"), nil
}
