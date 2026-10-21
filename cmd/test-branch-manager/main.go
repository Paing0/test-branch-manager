// Git Branch Manager - Automates the offshore and IT test branch workflow.
//
// This tool creates test/offshore-test-{date} and test/it-{date} branches
// from a develop base, merging selected source branches and handling
// conflicts interactively.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"test-branch-creator/internal/cli"
	"test-branch-creator/internal/config"
	"test-branch-creator/internal/history"
	"test-branch-creator/internal/workflow"
)

var version = "dev"

func main() {
	exitCode := run()
	os.Exit(exitCode)
}

func run() int {
	// Parse flags
	var (
		verbose    bool
		logFile    string
		workDir    string
		showHelp   bool
		showVer    bool
		accessible bool
	)

	flag.BoolVar(&verbose, "verbose", false, "Show detailed Git command output")
	flag.BoolVar(&verbose, "v", false, "Show detailed Git command output (shorthand)")
	flag.StringVar(&logFile, "log-file", "", "Path to save command history log")
	flag.StringVar(&workDir, "dir", "", "Working directory to scan for repositories (default: current directory)")
	flag.BoolVar(&showHelp, "help", false, "Show help message")
	flag.BoolVar(&showHelp, "h", false, "Show help message (shorthand)")
	flag.BoolVar(&showVer, "version", false, "Show version")
	flag.BoolVar(&accessible, "accessible", false, "Use accessible mode (no fancy UI)")
	flag.Parse()

	if showHelp {
		printUsage()
		return 0
	}

	if showVer {
		fmt.Printf("test-branch-manager version %s\n", version)
		return 0
	}

	// Determine working directory
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to get current directory: %v\n", err)
			return 1
		}
	}

	// Create the history recorder — summary is ALWAYS printed on exit
	recorder := history.NewRecorder()

	// Set up signal handling with context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		fmt.Fprintf(os.Stderr, "\nReceived signal: %v\n", sig)
		recorder.Workflow.Status = "Interrupted by signal"
		cancel()
	}()

	// Always print summary on exit
	defer func() {
		recorder.PrintSummary(os.Stderr)

		if logFile != "" {
			if err := recorder.WriteToFile(logFile); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to write log file: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "Command log saved to: %s\n", logFile)
			}
		}
	}()

	// Create the configuration
	cfg := config.New(workDir, verbose, logFile)

	// Create UI components
	ui := cli.NewUI()

	var prompter cli.Prompter
	if accessible {
		prompter = cli.NewAccessiblePrompter()
	} else {
		prompter = cli.NewPrompter()
	}

	// Create and run the workflow
	wf := workflow.New(cfg, ui, prompter, recorder)

	if err := wf.Run(ctx); err != nil {
		ui.Blank()
		ui.Errorf("Workflow failed: %v", err)
		return 1
	}

	return 0
}

func printUsage() {
	fmt.Println(`Git Branch Manager - Automated offshore and IT test branch workflow

USAGE:
  test-branch-manager [flags]

DESCRIPTION:
  Automates the creation of test/offshore-test-{date} and test/it-{date}
  branches from a develop base, with interactive source branch selection
  and conflict resolution.

WORKFLOW:
  Phase A: Create offshore test branch from develop
  Phase B: Merge source branch (and optional IT branch) into offshore branch
  Phase C: Create IT test branch from develop, merge offshore into it

FLAGS:
  -dir <path>       Working directory to scan for repositories (default: current dir)
  -v, -verbose      Show detailed Git command output
  -log-file <path>  Save command history to a file
  -accessible       Use accessible mode (no fancy interactive UI)
  -version          Show version
  -h, -help         Show this help message

EXAMPLES:
  test-branch-manager                       Run in current directory
  test-branch-manager -dir /path/to/repos   Scan specific directory
  test-branch-manager -v -log-file run.log  Verbose mode with log file
  test-branch-manager -accessible           Non-interactive mode`)
}
