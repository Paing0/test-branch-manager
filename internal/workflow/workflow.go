// Package workflow implements the Git branch management workflow.
package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"test-branch-creator/internal/cli"
	"test-branch-creator/internal/config"
	"test-branch-creator/internal/git"
	"test-branch-creator/internal/history"
)

// Workflow orchestrates the complete branch management process.
type Workflow struct {
	cfg      *config.Config
	ui       *cli.UI
	prompter cli.Prompter
	recorder *history.Recorder
	executor *git.Executor

	// Workflow state
	repo             *git.Repository
	remote           string
	sourceBranch     string
	offshoreBranch   string
	itBranch         string
	mergeExistingIT  bool
	existingITBranch string
}

// New creates a new Workflow instance.
func New(cfg *config.Config, ui *cli.UI, prompter cli.Prompter, recorder *history.Recorder) *Workflow {
	return &Workflow{
		cfg:      cfg,
		ui:       ui,
		prompter: prompter,
		recorder: recorder,
		executor: git.NewExecutor(recorder, cfg.Verbose),
	}
}

// Run executes the full branch management workflow.
func (w *Workflow) Run(ctx context.Context) error {
	w.recorder.Workflow.Status = "In progress"

	w.ui.Header("Git Branch Manager")
	w.ui.Infof("Date: %s", w.cfg.DateStr)
	w.ui.Infof("Offshore branch: %s", w.ui.BranchName(w.cfg.OffshoreBranchName()))
	w.ui.Infof("IT test branch:  %s", w.ui.BranchName(w.cfg.ITBranchName()))
	w.ui.Blank()

	// Step 1: Select repository
	if err := w.selectRepository(ctx); err != nil {
		return w.handleError("Repository selection", err)
	}

	// Step 2: Validate repository state
	if err := w.validateRepository(ctx); err != nil {
		return w.handleError("Repository validation", err)
	}

	// Step 3: Select remote
	if err := w.selectRemote(ctx); err != nil {
		return w.handleError("Remote selection", err)
	}

	// Phase A: Prepare the offshore test branch
	if err := w.phaseA(ctx); err != nil {
		return w.handleError("Phase A (offshore branch preparation)", err)
	}

	// Phase B: Merge into the offshore test branch
	if err := w.phaseB(ctx); err != nil {
		return w.handleError("Phase B (offshore branch merges)", err)
	}

	// Phase C: Create and prepare IT test branch
	if err := w.phaseC(ctx); err != nil {
		return w.handleError("Phase C (IT test branch)", err)
	}

	w.recorder.Workflow.Status = "Completed successfully"
	w.printFinalSummary()
	return nil
}

// selectRepository prompts the user to choose a Git repository.
func (w *Workflow) selectRepository(ctx context.Context) error {
	w.ui.Step("Setup", "Discovering Git repositories...")

	repos, err := git.DiscoverRepos(w.cfg.WorkDir)
	if err != nil {
		return fmt.Errorf("failed to scan for repositories: %w", err)
	}

	if len(repos) == 0 {
		return fmt.Errorf("no Git repositories found in %s\n\nMake sure you run this tool from a directory containing Git repositories", w.cfg.WorkDir)
	}

	items := make([]cli.SelectItem, 0, len(repos))
	for _, repo := range repos {
		items = append(items, cli.SelectItem{
			Label: fmt.Sprintf("%s  %s", repo.Name, w.ui.Dim(repo.Path)),
			Value: repo.Path,
		})
	}

	w.ui.Infof("Found %d Git repository(ies)", len(repos))

	selectedPath, err := w.prompter.SelectOne("Select a Git repository:", items)
	if err != nil {
		return err
	}

	w.repo = git.NewRepository(selectedPath, w.executor)
	w.recorder.Workflow.Repository = w.repo.Name
	w.recorder.Workflow.RepositoryPath = w.repo.Path

	w.ui.Successf("Selected repository: %s", w.repo.Name)
	return nil
}

// validateRepository checks the repository state is safe for the workflow.
func (w *Workflow) validateRepository(ctx context.Context) error {
	w.ui.Step("Setup", "Validating repository state...")
	if err := w.repo.Validate(ctx); err != nil {
		return err
	}
	w.ui.Success("Repository state is clean")
	return nil
}

// selectRemote determines which remote to use.
func (w *Workflow) selectRemote(ctx context.Context) error {
	w.ui.Step("Setup", "Checking remote configuration...")

	exists, err := w.repo.RemoteExists(ctx, w.cfg.Remote)
	if err != nil {
		return err
	}

	if exists {
		w.remote = w.cfg.Remote
		w.recorder.Workflow.Remote = w.remote
		w.ui.Successf("Using remote: %s", w.remote)
		return nil
	}

	w.ui.Warningf("Default remote '%s' not found", w.cfg.Remote)

	remotes, err := w.repo.ListRemotes(ctx)
	if err != nil {
		return err
	}

	if len(remotes) == 0 {
		return fmt.Errorf("no remotes configured; please add a remote first with 'git remote add'")
	}

	items := make([]cli.SelectItem, 0, len(remotes))
	for _, r := range remotes {
		items = append(items, cli.SelectItem{
			Label: r,
			Value: r,
		})
	}

	selected, err := w.prompter.SelectOne("Select a remote:", items)
	if err != nil {
		return err
	}

	w.remote = selected
	w.cfg.Remote = selected
	w.recorder.Workflow.Remote = w.remote
	w.ui.Successf("Using remote: %s", w.remote)
	return nil
}

// phaseA prepares the offshore test branch.
func (w *Workflow) phaseA(ctx context.Context) error {
	w.ui.Header("Phase A: Prepare Offshore Test Branch")

	w.offshoreBranch = w.cfg.OffshoreBranchName()
	w.recorder.Workflow.OffshoreBranch = w.offshoreBranch

	// A1: Fetch remote
	w.ui.Step("A.1", "Fetching latest from remote...")
	if err := w.repo.Fetch(ctx, w.remote); err != nil {
		return err
	}
	w.ui.Success("Fetch complete")

	// A2: Select source branch
	w.ui.Step("A.2", "Loading remote branches...")
	branches, err := w.repo.ListRemoteBranches(ctx, w.remote)
	if err != nil {
		return err
	}

	if len(branches) == 0 {
		return fmt.Errorf("no remote branches found on '%s'", w.remote)
	}

	items := make([]cli.SelectItem, 0, len(branches))
	for _, b := range branches {
		items = append(items, cli.SelectItem{
			Label: b,
			Value: b,
		})
	}

	w.ui.Infof("Found %d remote branches", len(branches))
	selectedBranch, err := w.prompter.SelectOne("Select the source branch to merge into the offshore branch:", items)
	if err != nil {
		return err
	}
	w.sourceBranch = selectedBranch
	w.recorder.Workflow.SourceBranch = w.sourceBranch
	w.ui.Successf("Source branch: %s", w.ui.BranchName(w.sourceBranch))

	// A3: Verify develop exists
	w.ui.Step("A.3", "Verifying 'develop' branch exists on remote...")
	developRef := w.remote + "/develop"
	developExists, err := w.repo.RemoteBranchExists(ctx, w.remote, "develop")
	if err != nil {
		return fmt.Errorf("failed to check for develop branch: %w", err)
	}
	if !developExists {
		return fmt.Errorf("branch 'develop' does not exist on remote '%s'; this workflow requires a develop branch", w.remote)
	}
	w.ui.Successf("Confirmed: %s exists", w.ui.BranchName(developRef))

	// A4: Check if offshore branch already exists
	w.ui.Step("A.4", "Checking if offshore branch already exists...")
	localExists, remoteExists, err := w.repo.BranchExistsAnywhere(ctx, w.offshoreBranch, w.remote)
	if err != nil {
		return err
	}

	if localExists || remoteExists {
		location := "locally"
		if remoteExists {
			location = "on remote"
		}
		if localExists && remoteExists {
			location = "locally and on remote"
		}
		w.ui.Warningf("Branch '%s' already exists %s", w.offshoreBranch, location)

		proceed, err := w.prompter.Confirm(
			fmt.Sprintf("Branch '%s' already exists. Would you like to cancel and choose a different date?", w.offshoreBranch),
			true,
		)
		if err != nil {
			return err
		}
		if proceed {
			return fmt.Errorf("branch '%s' already exists; please choose a different date suffix or delete the existing branch", w.offshoreBranch)
		}
	}

	// A5: Create offshore branch from develop
	w.ui.Step("A.5", fmt.Sprintf("Creating branch %s from %s...", w.ui.BranchName(w.offshoreBranch), w.ui.BranchName(developRef)))
	if err := w.repo.CreateBranchFrom(ctx, w.offshoreBranch, developRef); err != nil {
		return err
	}
	w.ui.Successf("Created and checked out %s", w.ui.BranchName(w.offshoreBranch))

	// A6: Ask about merging an existing IT test branch
	w.ui.Step("A.6", "Checking for existing IT test branches...")
	mergeIT, err := w.prompter.Confirm(
		"Do you want to merge an existing IT test branch into the offshore branch?",
		false,
	)
	if err != nil {
		return err
	}

	if mergeIT {
		itBranches, err := w.repo.ListRemoteBranchesMatching(ctx, w.remote, "test/it-*")
		if err != nil {
			return err
		}

		if len(itBranches) == 0 {
			w.ui.Warning("No existing IT test branches found on remote")
			w.mergeExistingIT = false
		} else {
			itItems := make([]cli.SelectItem, 0, len(itBranches))
			for _, b := range itBranches {
				itItems = append(itItems, cli.SelectItem{
					Label: b,
					Value: b,
				})
			}

			selectedIT, err := w.prompter.SelectOne("Select an existing IT test branch to merge:", itItems)
			if err != nil {
				return err
			}
			w.existingITBranch = selectedIT
			w.mergeExistingIT = true
			w.ui.Successf("Will merge existing IT branch: %s", w.ui.BranchName(w.existingITBranch))
		}
	} else {
		w.mergeExistingIT = false
		w.ui.Info("Skipping existing IT test branch merge")
	}

	w.ui.Blank()
	w.ui.Success("Phase A complete — offshore branch prepared")
	return nil
}

// phaseB merges branches into the offshore test branch.
func (w *Workflow) phaseB(ctx context.Context) error {
	w.ui.Header("Phase B: Merge into Offshore Test Branch")

	// B1: Merge source branch
	w.ui.Step("B.1", fmt.Sprintf("Merging %s into %s...", w.ui.BranchName(w.sourceBranch), w.ui.BranchName(w.offshoreBranch)))

	if err := w.mergeWithConflictLoop(ctx, w.sourceBranch); err != nil {
		return err
	}
	w.ui.Successf("Merged %s successfully", w.ui.BranchName(w.sourceBranch))

	// B2: Optionally merge existing IT test branch
	if w.mergeExistingIT {
		w.ui.Step("B.2", fmt.Sprintf("Merging %s into %s...", w.ui.BranchName(w.existingITBranch), w.ui.BranchName(w.offshoreBranch)))

		if err := w.mergeWithConflictLoop(ctx, w.existingITBranch); err != nil {
			return err
		}
		w.ui.Successf("Merged %s successfully", w.ui.BranchName(w.existingITBranch))
	}

	// B3: Push offshore branch
	w.ui.Step("B.3", fmt.Sprintf("Pushing %s to %s...", w.ui.BranchName(w.offshoreBranch), w.remote))

	pushConfirmed, err := w.prompter.Confirm(
		fmt.Sprintf("Push '%s' to remote '%s'?", w.offshoreBranch, w.remote),
		true,
	)
	if err != nil {
		return err
	}
	if !pushConfirmed {
		return cli.ErrUserCancelled
	}

	if err := w.repo.Push(ctx, w.remote, w.offshoreBranch); err != nil {
		w.recorder.Workflow.RecoveryMessages = append(w.recorder.Workflow.RecoveryMessages,
			"Check your network connection and remote credentials",
			fmt.Sprintf("Try pushing manually: git push %s %s", w.remote, w.offshoreBranch),
		)
		return err
	}
	w.ui.Successf("Pushed %s to %s", w.ui.BranchName(w.offshoreBranch), w.remote)

	w.ui.Blank()
	w.ui.Success("Phase B complete — offshore branch merged and pushed")
	return nil
}

// phaseC creates the IT test branch.
func (w *Workflow) phaseC(ctx context.Context) error {
	w.ui.Header("Phase C: Create IT Test Branch")

	w.itBranch = w.cfg.ITBranchName()
	w.recorder.Workflow.ITBranch = w.itBranch

	developRef := w.remote + "/develop"

	// C1: Check if IT branch already exists
	w.ui.Step("C.1", "Checking if IT test branch already exists...")
	localExists, remoteExists, err := w.repo.BranchExistsAnywhere(ctx, w.itBranch, w.remote)
	if err != nil {
		return err
	}

	if localExists || remoteExists {
		location := "locally"
		if remoteExists {
			location = "on remote"
		}
		if localExists && remoteExists {
			location = "locally and on remote"
		}
		w.ui.Warningf("Branch '%s' already exists %s", w.itBranch, location)

		proceed, err := w.prompter.Confirm(
			fmt.Sprintf("Branch '%s' already exists. Would you like to cancel?", w.itBranch),
			true,
		)
		if err != nil {
			return err
		}
		if proceed {
			return fmt.Errorf("branch '%s' already exists; please choose a different date suffix or delete the existing branch", w.itBranch)
		}
	}

	// C2: Fetch latest develop (to ensure we have the latest)
	w.ui.Step("C.2", "Fetching latest develop...")
	if err := w.repo.Fetch(ctx, w.remote); err != nil {
		return err
	}

	// C3: Create IT branch from develop
	w.ui.Step("C.3", fmt.Sprintf("Creating branch %s from %s...", w.ui.BranchName(w.itBranch), w.ui.BranchName(developRef)))
	if err := w.repo.CreateBranchFrom(ctx, w.itBranch, developRef); err != nil {
		return err
	}
	w.ui.Successf("Created and checked out %s", w.ui.BranchName(w.itBranch))

	// C4: Merge offshore branch into IT branch
	w.ui.Step("C.4", fmt.Sprintf("Merging %s into %s...", w.ui.BranchName(w.offshoreBranch), w.ui.BranchName(w.itBranch)))

	if err := w.mergeWithConflictLoop(ctx, w.offshoreBranch); err != nil {
		return err
	}
	w.ui.Successf("Merged %s successfully", w.ui.BranchName(w.offshoreBranch))

	// C5: Push IT branch
	w.ui.Step("C.5", fmt.Sprintf("Pushing %s to %s...", w.ui.BranchName(w.itBranch), w.remote))

	pushConfirmed, err := w.prompter.Confirm(
		fmt.Sprintf("Push '%s' to remote '%s'?", w.itBranch, w.remote),
		true,
	)
	if err != nil {
		return err
	}
	if !pushConfirmed {
		return cli.ErrUserCancelled
	}

	if err := w.repo.Push(ctx, w.remote, w.itBranch); err != nil {
		w.recorder.Workflow.RecoveryMessages = append(w.recorder.Workflow.RecoveryMessages,
			"Check your network connection and remote credentials",
			fmt.Sprintf("Try pushing manually: git push %s %s", w.remote, w.itBranch),
		)
		return err
	}
	w.ui.Successf("Pushed %s to %s", w.ui.BranchName(w.itBranch), w.remote)

	w.ui.Blank()
	w.ui.Success("Phase C complete — IT test branch created and pushed")
	return nil
}

// mergeWithConflictLoop performs a merge with interactive conflict resolution.
func (w *Workflow) mergeWithConflictLoop(ctx context.Context, branch string) error {
	result, err := w.repo.Merge(ctx, branch)
	if err != nil {
		return err
	}

	if result.Success {
		return nil
	}

	if !result.HasConflicts {
		return fmt.Errorf("merge failed: %s", result.Message)
	}

	// Conflict resolution loop
	for {
		w.ui.Warning("Merge conflicts detected!")
		w.ui.FileList("Conflicted files:", result.ConflictedFiles)

		w.ui.Print("Please resolve the conflicts using your preferred editor or Git tools.")
		w.ui.Print("After resolving:")
		w.ui.Print("  1. Edit the conflicted files to resolve the conflicts")
		w.ui.Print("  2. Stage the resolved files with 'git add <file>'")
		w.ui.Print("  3. Come back here and confirm")
		w.ui.Blank()

		err := w.prompter.WaitForUser("Press Continue when you have resolved all conflicts, or Cancel to stop")
		if err != nil {
			w.recorder.Workflow.Status = "Interrupted by unresolved merge conflicts"
			w.recorder.Workflow.RecoveryMessages = append(w.recorder.Workflow.RecoveryMessages,
				"Resolve the listed conflicts in your editor",
				"Stage the resolved files with 'git add <file>'",
				"Complete the merge commit with 'git commit'",
				"Rerun this application to continue the workflow",
			)
			return fmt.Errorf("merge interrupted: conflicts remain unresolved")
		}

		// Recheck conflicts
		conflictedFiles, err := w.repo.GetConflictedFiles(ctx)
		if err != nil {
			return fmt.Errorf("failed to check conflict state: %w", err)
		}

		if len(conflictedFiles) > 0 {
			w.ui.Warning("Some conflicts are still unresolved!")
			result.ConflictedFiles = conflictedFiles
			continue
		}

		// Check if we need to complete the merge commit
		needsCommit, err := w.repo.NeedsMergeCommit(ctx)
		if err != nil {
			return fmt.Errorf("failed to check merge state: %w", err)
		}

		if needsCommit {
			w.ui.Info("Completing merge commit...")
			if err := w.repo.CommitMerge(ctx); err != nil {
				// If commit fails, user might need to stage files first
				w.ui.Warning("Could not auto-complete the merge commit")
				w.ui.Print("Try staging all resolved files and committing:")
				w.ui.Print("  git add -A")
				w.ui.Print("  git commit --no-edit")

				stageAndCommit, promptErr := w.prompter.Confirm("Would you like to stage all files and complete the commit?", true)
				if promptErr != nil {
					return promptErr
				}
				if stageAndCommit {
					if stageErr := w.repo.StageAll(ctx); stageErr != nil {
						return fmt.Errorf("failed to stage files: %w", stageErr)
					}
					if commitErr := w.repo.CommitMerge(ctx); commitErr != nil {
						return fmt.Errorf("failed to complete merge: %w", commitErr)
					}
				} else {
					return fmt.Errorf("merge commit not completed; please complete it manually and rerun")
				}
			}
		}

		// Verify merge is complete
		stillMerging, err := w.repo.IsMergeInProgress(ctx)
		if err != nil {
			return fmt.Errorf("failed to verify merge state: %w", err)
		}
		if stillMerging {
			w.ui.Warning("Merge is still in progress. Please complete the merge commit.")
			continue
		}

		w.ui.Success("Conflicts resolved and merge completed")
		return nil
	}
}

// handleError sets the workflow status and returns the error.
func (w *Workflow) handleError(phase string, err error) error {
	if strings.Contains(w.recorder.Workflow.Status, "conflict") {
		return err
	}
	if errors.Is(err, cli.ErrUserCancelled) {
		w.recorder.Workflow.Status = fmt.Sprintf("Cancelled by user during %s", phase)
	} else {
		w.recorder.Workflow.Status = fmt.Sprintf("Failed during %s", phase)
		w.recorder.Workflow.RecoveryMessages = append(w.recorder.Workflow.RecoveryMessages,
			"Check the error message above for details",
			"Verify the repository state with 'git status'",
			"Rerun the application after fixing the issue",
		)
	}
	return err
}

// printFinalSummary shows the success summary.
func (w *Workflow) printFinalSummary() {
	w.ui.Header("Workflow Complete!")
	w.ui.SummaryLine("Repository", fmt.Sprintf("%s (%s)", w.repo.Name, w.repo.Path))
	w.ui.SummaryLine("Source branch", w.sourceBranch)
	w.ui.SummaryLine("Offshore branch", w.offshoreBranch)
	w.ui.SummaryLine("IT test branch", w.itBranch)
	w.ui.SummaryLine("Remote", w.remote)
	w.ui.Blank()
}
