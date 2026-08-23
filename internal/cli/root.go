package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/egekocabas/kick-sim/internal/app"
	"github.com/egekocabas/kick-sim/internal/workspace"
	"github.com/spf13/cobra"
)

const (
	exitGeneral   = 1
	exitUsage     = 2
	exitWorkspace = 3
	exitScenario  = 4
	exitDelivery  = 5
)

type exitError struct {
	code int
	err  error
}

func (err *exitError) Error() string { return err.err.Error() }
func (err *exitError) Unwrap() error { return err.err }

type environment struct {
	stdout        io.Writer
	stderr        io.Writer
	workspaceFlag string
	output        string
	verbose       bool
	start         string
}

func Execute() int {
	command := NewRootCommand(os.Stdout, os.Stderr)
	if err := command.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return errorCode(err)
	}
	return 0
}

func NewRootCommand(stdout, stderr io.Writer) *cobra.Command {
	environment := &environment{stdout: stdout, stderr: stderr}
	root := &cobra.Command{
		Use:           "kick-sim",
		Short:         "Unofficial local simulator for Kick webhook integrations",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().StringVar(&environment.workspaceFlag, "workspace", "", "workspace directory")
	root.PersistentFlags().StringVarP(&environment.output, "output", "o", "human", "output format: human or json")
	root.PersistentFlags().BoolVarP(&environment.verbose, "verbose", "v", false, "show resolved workspace details")
	root.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		if environment.output != "human" && environment.output != "json" {
			return usageError(fmt.Errorf("unsupported output format %q", environment.output))
		}
		return nil
	}
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return usageError(err) })

	root.AddCommand(newInitCommand(environment))
	root.AddCommand(newWorkspaceCommand(environment))
	root.AddCommand(newEventCommand(environment))
	root.AddCommand(newScenarioCommand(environment))
	root.AddCommand(newKeysCommand(environment))
	root.AddCommand(newConfigCommand(environment))
	root.AddCommand(newCompatibilityCommand(environment))
	root.AddCommand(newVersionCommand(environment))
	return root
}

func (environment *environment) resolve(initialize bool) (string, error) {
	root, err := workspace.Resolve(workspace.ResolveOptions{
		Explicit:   environment.workspaceFlag,
		Start:      environment.start,
		Initialize: initialize,
	})
	if err != nil {
		return "", workspaceError(err)
	}
	if environment.verbose && environment.output == "human" {
		fmt.Fprintf(environment.stderr, "Workspace: %s\n", root)
	}
	return root, nil
}

func (environment *environment) service() (*app.Service, error) {
	root, err := environment.resolve(false)
	if err != nil {
		return nil, err
	}
	service, err := app.Open(root)
	if err != nil {
		return nil, workspaceError(err)
	}
	return service, nil
}

func (environment *environment) writeJSON(value any) error {
	encoder := json.NewEncoder(environment.stdout)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func usageError(err error) error     { return &exitError{code: exitUsage, err: err} }
func workspaceError(err error) error { return &exitError{code: exitWorkspace, err: err} }
func scenarioError(err error) error  { return &exitError{code: exitScenario, err: err} }
func deliveryError(err error) error  { return &exitError{code: exitDelivery, err: err} }

func errorCode(err error) int {
	var coded *exitError
	if errors.As(err, &coded) {
		return coded.code
	}
	message := err.Error()
	for _, marker := range []string{"unknown command", "accepts ", "requires at least", "requires at most"} {
		if strings.Contains(message, marker) {
			return exitUsage
		}
	}
	return exitGeneral
}

func executeForTest(ctx context.Context, command *cobra.Command, args ...string) error {
	command.SetArgs(args)
	return command.ExecuteContext(ctx)
}
