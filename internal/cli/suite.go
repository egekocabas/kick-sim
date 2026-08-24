package cli

import (
	"fmt"

	"github.com/egekocabas/kick-sim/internal/scenario"
	"github.com/egekocabas/kick-sim/internal/suite"
	"github.com/spf13/cobra"
)

func newSuiteCommand(environment *environment) *cobra.Command {
	command := &cobra.Command{Use: "suite", Short: "Validate and run scenario suites"}
	command.AddCommand(newSuiteListCommand(environment), newSuiteShowCommand(environment), newSuiteValidateCommand(environment), newSuiteRunCommand(environment))
	return command
}

func suiteStore(serviceEnvironment *environment) (*suite.Store, error) {
	service, err := serviceEnvironment.service()
	if err != nil {
		return nil, err
	}
	scenarios := scenario.NewStore(service.Workspace, service.Events, service.Config, service.Actors)
	return suite.NewStore(service.Workspace, scenarios), nil
}

func newSuiteListCommand(environment *environment) *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List built-in and custom suites", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		store, err := suiteStore(environment)
		if err != nil {
			return err
		}
		entries, err := store.List()
		if err != nil {
			return scenarioError(err)
		}
		if environment.output == "json" {
			return environment.writeJSON(entries)
		}
		for _, entry := range entries {
			kind := "custom"
			if entry.BuiltIn {
				kind = "built-in"
			}
			fmt.Fprintf(environment.stdout, "%s\t%s\t%s\n", entry.ID, kind, entry.Suite.Name)
		}
		return nil
	}}
}

func newSuiteShowCommand(environment *environment) *cobra.Command {
	return &cobra.Command{Use: "show <suite-id>", Short: "Show a suite definition", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		store, err := suiteStore(environment)
		if err != nil {
			return err
		}
		entry, err := store.Get(args[0])
		if err != nil {
			return scenarioError(err)
		}
		if environment.output == "json" {
			return environment.writeJSON(entry)
		}
		_, err = environment.stdout.Write(entry.Source)
		return err
	}}
}

func newSuiteValidateCommand(environment *environment) *cobra.Command {
	return &cobra.Command{Use: "validate <suite-id>", Short: "Validate a suite and referenced scenarios", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		store, err := suiteStore(environment)
		if err != nil {
			return err
		}
		entry, err := store.Get(args[0])
		if err != nil {
			return scenarioError(err)
		}
		if err := store.Validate(entry); err != nil {
			return scenarioError(err)
		}
		if environment.output == "json" {
			return environment.writeJSON(map[string]any{"id": entry.ID, "valid": true})
		}
		_, err = fmt.Fprintf(environment.stdout, "Suite %s is valid\n", entry.ID)
		return err
	}}
}

func newSuiteRunCommand(environment *environment) *cobra.Command {
	var deliveryOptions deliveryFlags
	command := &cobra.Command{Use: "run <suite-id>", Short: "Run a suite and enforce its thresholds", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		service, err := environment.service()
		if err != nil {
			return err
		}
		scenarios := scenario.NewStore(service.Workspace, service.Events, service.Config, service.Actors)
		store := suite.NewStore(service.Workspace, scenarios)
		entry, err := store.Get(args[0])
		if err != nil {
			return scenarioError(err)
		}
		if err := store.Validate(entry); err != nil {
			return scenarioError(err)
		}
		report, runErr := suite.Run(command.Context(), service, scenarios, entry, suite.RunOptions{Destination: deliveryOptions.destination, DestinationURL: deliveryOptions.destinationURL})
		if environment.output == "json" {
			if err := environment.writeJSON(report); err != nil {
				return err
			}
		} else {
			for _, result := range report.Cases {
				status := "PASS"
				if !result.Passed {
					status = "FAIL"
				}
				fmt.Fprintf(environment.stdout, "%s\t%s\t%s\n", status, result.ScenarioID, result.Error)
			}
			fmt.Fprintf(environment.stdout, "Suite %s: %d passed, %d failed (%.1f%%)\n", entry.ID, report.PassedCases, report.FailedCases, report.PassedPercent)
		}
		if runErr != nil {
			return deliveryError(runErr)
		}
		return nil
	}}
	deliveryOptions.register(command)
	return command
}
