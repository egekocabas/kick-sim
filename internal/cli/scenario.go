package cli

import (
	"fmt"

	"github.com/egekocabas/kick-sim/internal/scenario"
	"github.com/egekocabas/kick-sim/internal/version"
	"github.com/spf13/cobra"
)

func newScenarioCommand(environment *environment) *cobra.Command {
	command := &cobra.Command{Use: "scenario", Short: "Manage and run reusable webhook scenarios"}
	command.AddCommand(newScenarioListCommand(environment))
	command.AddCommand(newScenarioShowCommand(environment))
	command.AddCommand(newScenarioCopyCommand(environment))
	command.AddCommand(newScenarioValidateCommand(environment))
	command.AddCommand(newScenarioRunCommand(environment))
	return command
}

func newScenarioListCommand(environment *environment) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List built-in and custom scenarios",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			service, err := environment.service()
			if err != nil {
				return err
			}
			store := scenario.NewStore(service.Workspace, service.Events, service.Config)
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
				fmt.Fprintf(environment.stdout, "%s\t%s\t%s\n", entry.ID, kind, entry.Scenario.Name)
			}
			return nil
		},
	}
}

func newScenarioShowCommand(environment *environment) *cobra.Command {
	return &cobra.Command{
		Use:   "show <scenario-id>",
		Short: "Show a scenario definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			service, err := environment.service()
			if err != nil {
				return err
			}
			entry, err := scenario.NewStore(service.Workspace, service.Events, service.Config).Get(args[0])
			if err != nil {
				return scenarioError(err)
			}
			if environment.output == "json" {
				return environment.writeJSON(entry)
			}
			_, err = environment.stdout.Write(entry.Source)
			return err
		},
	}
}

func newScenarioCopyCommand(environment *environment) *cobra.Command {
	return &cobra.Command{
		Use:   "copy <source-id> <target-id>",
		Short: "Copy a scenario into the workspace",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			service, err := environment.service()
			if err != nil {
				return err
			}
			entry, err := scenario.NewStore(service.Workspace, service.Events, service.Config).Copy(args[0], args[1], "kick-sim@"+version.Version)
			if err != nil {
				return scenarioError(err)
			}
			if environment.output == "json" {
				return environment.writeJSON(entry)
			}
			_, err = fmt.Fprintf(environment.stdout, "Scenario copied to %s\n", entry.Path)
			return err
		},
	}
}

func newScenarioValidateCommand(environment *environment) *cobra.Command {
	return &cobra.Command{
		Use:   "validate <scenario-id>",
		Short: "Validate a scenario and its resolved event payload",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			service, err := environment.service()
			if err != nil {
				return err
			}
			store := scenario.NewStore(service.Workspace, service.Events, service.Config)
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
			_, err = fmt.Fprintf(environment.stdout, "Scenario %s is valid\n", entry.ID)
			return err
		},
	}
}

func newScenarioRunCommand(environment *environment) *cobra.Command {
	var payload payloadFlags
	var deliveryOptions deliveryFlags
	command := &cobra.Command{
		Use:   "run <scenario-id>",
		Short: "Generate and deliver a scenario",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			service, err := environment.service()
			if err != nil {
				return err
			}
			store := scenario.NewStore(service.Workspace, service.Events, service.Config)
			entry, err := store.Get(args[0])
			if err != nil {
				return scenarioError(err)
			}
			if err := store.Validate(entry); err != nil {
				return scenarioError(err)
			}
			options, err := payload.options(
				command,
				entry.Scenario.Request.Event.Type,
				entry.Scenario.Request.Event.Version,
				entry.Scenario.Request.Payload,
				entry.Scenario.Request.Omit,
			)
			if err != nil {
				return usageError(err)
			}
			subscriptionID := deliveryOptions.subscriptionID
			if subscriptionID == "" {
				subscriptionID = entry.Scenario.Request.Delivery.SubscriptionID
			}
			generated, err := service.Generate(options, subscriptionID)
			if err != nil {
				return scenarioError(err)
			}
			destination := deliveryOptions.destination
			if destination == "" {
				destination = entry.Scenario.Request.Delivery.Destination
			}
			result, err := service.Deliver(
				command.Context(),
				generated,
				destination,
				deliveryOptions.destinationURL,
				entry.Scenario.Request.Delivery.Expect.Statuses,
			)
			if err != nil {
				return deliveryError(err)
			}
			return writeRunResult(environment, result)
		},
	}
	payload.register(command)
	deliveryOptions.register(command)
	return command
}
