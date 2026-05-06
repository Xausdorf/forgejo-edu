package edu

import (
	"context"
	"fmt"

	"forgejo.org/models"
	git_model "forgejo.org/models/git"
	org_model "forgejo.org/models/organization"
	"forgejo.org/models/perm"
	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/git"
	"forgejo.org/modules/optional"
	repo_module "forgejo.org/modules/repository"
	"forgejo.org/services/repository"
)

// ForgejoAdapter implements RepoForker using Forgejo's internal services.
type ForgejoAdapter struct{}

// NewForgejoAdapter creates a new ForgejoAdapter.
func NewForgejoAdapter() *ForgejoAdapter {
	return &ForgejoAdapter{}
}

// ForkRepositoryAndUpdates calls the Forgejo service to fork a repository.
func (a *ForgejoAdapter) ForkRepositoryAndUpdates(ctx context.Context, doer, owner *user_model.User, opts ForkRepoOptions) (*repo_model.Repository, error) {
	// Forgejo's fork logic accesses BaseRepo.Owner.Visibility,
	// so Owner must be loaded before calling.
	if opts.BaseRepo.Owner == nil {
		if err := opts.BaseRepo.LoadOwner(ctx); err != nil {
			return nil, fmt.Errorf("load repo owner: %w", err)
		}
	}
	serviceOpts := repository.ForkRepoOptions{
		BaseRepo:    opts.BaseRepo,
		Name:        opts.Name,
		Description: opts.BaseRepo.Description,
	}
	return repository.ForkRepositoryAndUpdates(ctx, doer, owner, serviceOpts)
}

// GetRepositoryByID retrieves a repository by ID.
func (a *ForgejoAdapter) GetRepositoryByID(ctx context.Context, id int64) (*repo_model.Repository, error) {
	return repo_model.GetRepositoryByID(ctx, id)
}

// GetRepositoryByOwnerAndName returns the repo with the given name owned by the user/org with ownerID.
// Returns (nil, nil) if not found.
func (a *ForgejoAdapter) GetRepositoryByOwnerAndName(ctx context.Context, ownerID int64, repoName string) (*repo_model.Repository, error) {
	repo, err := repo_model.GetRepositoryByName(ctx, ownerID, repoName)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return repo, nil
}

// CreateUser creates a new Forgejo user via admin API.
func (a *ForgejoAdapter) CreateUser(ctx context.Context, username, email, password, fullName string) error {
	u := &user_model.User{
		Name:               username,
		Email:              email,
		Passwd:             password,
		FullName:           fullName,
		MustChangePassword: true,
	}
	overwrite := &user_model.CreateUserOverwriteOptions{
		IsActive: optional.Some(true),
	}
	return user_model.AdminCreateUser(ctx, u, overwrite)
}

// GetUserByName retrieves a Forgejo user by username.
func (a *ForgejoAdapter) GetUserByName(ctx context.Context, name string) (*user_model.User, error) {
	return user_model.GetUserByName(ctx, name)
}

// GetUserByID retrieves a Forgejo user by ID.
func (a *ForgejoAdapter) GetUserByID(ctx context.Context, id int64) (*user_model.User, error) {
	return user_model.GetUserByID(ctx, id)
}

// GetUserByEmail retrieves a Forgejo user by email address.
func (a *ForgejoAdapter) GetUserByEmail(ctx context.Context, email string) (*user_model.User, error) {
	return user_model.GetUserByEmail(ctx, email)
}

// SyncFork pushes changes from the base repo to a fork using InternalPushingEnvironment
// to bypass branch protection.
func (a *ForgejoAdapter) SyncFork(ctx context.Context, doer *user_model.User, forkRepo *repo_model.Repository, branch string) error {
	if err := forkRepo.GetBaseRepo(ctx); err != nil {
		return fmt.Errorf("get base repo: %w", err)
	}

	return git.Push(ctx, forkRepo.BaseRepo.RepoPath(), git.PushOptions{
		Remote: forkRepo.RepoPath(),
		Branch: fmt.Sprintf("%s:%s", branch, branch),
		Env:    repo_module.InternalPushingEnvironment(doer, forkRepo),
	})
}

// Force-push: dstBranch is server-managed; any divergent commits on it are
// intentionally overwritten.
func (a *ForgejoAdapter) PushBranchToFork(ctx context.Context, doer *user_model.User, forkRepo *repo_model.Repository, srcBranch, dstBranch string) error {
	if err := forkRepo.GetBaseRepo(ctx); err != nil {
		return fmt.Errorf("get base repo: %w", err)
	}
	return git.Push(ctx, forkRepo.BaseRepo.RepoPath(), git.PushOptions{
		Remote: forkRepo.RepoPath(),
		Branch: fmt.Sprintf("+%s:%s", srcBranch, dstBranch),
		Env:    repo_module.InternalPushingEnvironment(doer, forkRepo),
		Force:  true,
	})
}

// GetDefaultBranch returns the default branch name for a repository.
func (a *ForgejoAdapter) GetDefaultBranch(ctx context.Context, repoID int64) (string, error) {
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return "", err
	}
	return repo.DefaultBranch, nil
}

// EnsureTeam gets or creates a team in the given org with the specified access mode.
func (a *ForgejoAdapter) EnsureTeam(ctx context.Context, orgID int64, teamName string, accessMode perm.AccessMode) (*org_model.Team, error) {
	team, err := org_model.GetTeam(ctx, orgID, teamName)
	if err == nil {
		return team, nil
	}
	if !org_model.IsErrTeamNotExist(err) {
		return nil, fmt.Errorf("get team: %w", err)
	}

	// Team doesn't exist, create it
	team = &org_model.Team{
		OrgID:                   orgID,
		Name:                    teamName,
		AccessMode:              accessMode,
		IncludesAllRepositories: true,
	}
	if err := models.NewTeam(ctx, team); err != nil {
		if org_model.IsErrTeamAlreadyExist(err) {
			// Race condition: another goroutine created it
			return org_model.GetTeam(ctx, orgID, teamName)
		}
		return nil, fmt.Errorf("create team: %w", err)
	}
	return team, nil
}

// AddTeamMember adds a user to a team (idempotent).
func (a *ForgejoAdapter) AddTeamMember(ctx context.Context, team *org_model.Team, userID int64) error {
	return models.AddTeamMember(ctx, team, userID)
}

// RemoveTeamMember removes a user from a team.
func (a *ForgejoAdapter) RemoveTeamMember(ctx context.Context, team *org_model.Team, userID int64) error {
	return models.RemoveTeamMember(ctx, team, userID)
}

// GetTeam retrieves a team by org ID and name.
func (a *ForgejoAdapter) GetTeam(ctx context.Context, orgID int64, name string) (*org_model.Team, error) {
	return org_model.GetTeam(ctx, orgID, name)
}

// BranchExists checks whether the given branch exists in the repo.
func (a *ForgejoAdapter) BranchExists(ctx context.Context, repoID int64, branchName string) (bool, error) {
	_, err := git_model.GetBranch(ctx, repoID, branchName)
	if err != nil {
		if git_model.IsErrBranchNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// AddCollaborator adds the user as a collaborator on the repo with the given access mode.
func (a *ForgejoAdapter) AddCollaborator(ctx context.Context, repoID, userID int64, mode perm.AccessMode) error {
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return err
	}
	user, err := user_model.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := repo_module.AddCollaborator(ctx, repo, user); err != nil {
		return err
	}
	return repo_model.ChangeCollaborationAccessMode(ctx, repo, userID, mode)
}

// RemoveCollaborator removes a user as collaborator from the repo.
func (a *ForgejoAdapter) RemoveCollaborator(ctx context.Context, repoID, userID int64) error {
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return err
	}
	return repository.DeleteCollaboration(ctx, repo, userID)
}

// ProtectMainBranch enables branch protection on the named branch (CanPush=false, no merge whitelist).
func (a *ForgejoAdapter) ProtectMainBranch(ctx context.Context, repoID int64, branchName string) error {
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return err
	}

	existing, err := git_model.GetProtectedBranchRuleByName(ctx, repoID, branchName)
	if err != nil {
		return err
	}
	if existing == nil {
		existing = &git_model.ProtectedBranch{
			RepoID:   repoID,
			RuleName: branchName,
		}
	}
	existing.CanPush = false
	existing.EnableMergeWhitelist = false
	existing.RequireSignedCommits = false

	return git_model.UpdateProtectBranch(ctx, repo, existing, git_model.WhitelistOptions{})
}
