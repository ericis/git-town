package cmd

import (
	"fmt"

	"github.com/git-town/git-town/src/cli"
	"github.com/git-town/git-town/src/git"
	"github.com/git-town/git-town/src/prompt"
	"github.com/git-town/git-town/src/steps"

	"github.com/spf13/cobra"
)

type prependConfig struct {
	initialBranch       string
	parentBranch        string
	targetBranch        string
	ancestorBranches    []string
	hasOrigin           bool
	shouldNewBranchPush bool
	isOffline           bool
}

var prependCommand = &cobra.Command{
	Use:   "prepend <branch>",
	Short: "Creates a new feature branch as the parent of the current branch",
	Long: `Creates a new feature branch as the parent of the current branch

Syncs the parent branch,
cuts a new feature branch with the given name off the parent branch,
makes the new branch the parent of the current branch,
pushes the new feature branch to the remote repository
(if "new-branch-push-flag" is true),
and brings over all uncommitted changes to the new feature branch.

See "sync" for remote upstream options.
`,
	Run: func(cmd *cobra.Command, args []string) {
		config, err := getPrependConfig(args, prodRepo)
		if err != nil {
			cli.Exit(err)
		}
		stepList, err := getPrependStepList(config, prodRepo)
		if err != nil {
			cli.Exit(err)
		}
		runState := steps.NewRunState("prepend", stepList)
		err = steps.Run(runState, prodRepo, nil)
		if err != nil {
			fmt.Println(err)
			cli.Exit(err)
		}
	},
	Args: cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if err := ValidateIsRepository(prodRepo); err != nil {
			return err
		}
		return validateIsConfigured(prodRepo)
	},
}

func getPrependConfig(args []string, repo *git.ProdRepo) (result prependConfig, err error) {
	result.initialBranch, err = repo.Silent.CurrentBranch()
	if err != nil {
		return result, err
	}
	result.targetBranch = args[0]
	result.hasOrigin, err = repo.Silent.HasRemote("origin")
	if err != nil {
		return result, err
	}
	result.shouldNewBranchPush = repo.Config.ShouldNewBranchPush()
	result.isOffline = repo.Config.IsOffline()
	if result.hasOrigin && !result.isOffline {
		err := repo.Logging.Fetch()
		if err != nil {
			return result, err
		}
	}
	hasBranch, err := repo.Silent.HasLocalOrRemoteBranch(result.targetBranch)
	if err != nil {
		return result, err
	}
	if hasBranch {
		return result, fmt.Errorf("a branch named %q already exists", result.targetBranch)
	}
	if !repo.Config.IsFeatureBranch(result.initialBranch) {
		return result, fmt.Errorf("the branch %q is not a feature branch. Only feature branches can have parent branches", result.initialBranch)
	}
	err = prompt.EnsureKnowsParentBranches([]string{result.initialBranch}, repo)
	if err != nil {
		return result, err
	}
	result.parentBranch = repo.Config.GetParentBranch(result.initialBranch)
	result.ancestorBranches = repo.Config.GetAncestorBranches(result.initialBranch)
	return result, nil
}

func getPrependStepList(config prependConfig, repo *git.ProdRepo) (result steps.StepList, err error) {
	for _, branchName := range config.ancestorBranches {
		steps, err := steps.GetSyncBranchSteps(branchName, true, repo)
		if err != nil {
			return result, err
		}
		result.AppendList(steps)
	}
	result.Append(&steps.CreateBranchStep{BranchName: config.targetBranch, StartingPoint: config.parentBranch})
	result.Append(&steps.SetParentBranchStep{BranchName: config.targetBranch, ParentBranchName: config.parentBranch})
	result.Append(&steps.SetParentBranchStep{BranchName: config.initialBranch, ParentBranchName: config.targetBranch})
	result.Append(&steps.CheckoutBranchStep{BranchName: config.targetBranch})
	if config.hasOrigin && config.shouldNewBranchPush && !config.isOffline {
		result.Append(&steps.CreateTrackingBranchStep{BranchName: config.targetBranch})
	}
	err = result.Wrap(steps.WrapOptions{RunInGitRoot: true, StashOpenChanges: true}, repo)
	return result, err
}

func init() {
	RootCmd.AddCommand(prependCommand)
}
