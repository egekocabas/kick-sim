package cli

import (
	"fmt"

	"github.com/egekocabas/kick-sim/internal/scenario"
	"github.com/egekocabas/kick-sim/internal/version"
	"github.com/egekocabas/kick-sim/internal/workflow"
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
			store := scenario.NewStore(service.Workspace, service.Events, service.Config, service.Actors)
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
				name := entry.Scenario.Name
				if len(entry.ValidationErrors) > 0 {
					name = "invalid: " + entry.ValidationErrors[0]
				}
				fmt.Fprintf(environment.stdout, "%s\t%s\t%s\n", entry.ID, kind, name)
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
			entry, err := scenario.NewStore(service.Workspace, service.Events, service.Config, service.Actors).Get(args[0])
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
			entry, err := scenario.NewStore(service.Workspace, service.Events, service.Config, service.Actors).Copy(args[0], args[1], "kick-sim@"+version.Version)
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
			store := scenario.NewStore(service.Workspace, service.Events, service.Config, service.Actors)
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
			store := scenario.NewStore(service.Workspace, service.Events, service.Config, service.Actors)
			entry, err := store.Get(args[0])
			if err != nil {
				return scenarioError(err)
			}
			if err := store.Validate(entry); err != nil {
				return scenarioError(err)
			}
			if entry.Scenario.Kind() == "timeline" || entry.Scenario.Request.Delivery.Attempts > 1 {
				if entry.Scenario.Kind() == "timeline" {
					for _, name := range []string{"content", "sender", "sender-id", "broadcaster", "broadcaster-id", "set", "set-json", "unset"} {
						if command.Flags().Changed(name) {
							return usageError(fmt.Errorf("payload flag --%s is not supported for timeline scenarios; edit the timeline step payload", name))
						}
					}
				}
				report, runErr := workflow.Run(command.Context(), service, entry, workflow.Options{
					Destination: deliveryOptions.destination, DestinationURL: deliveryOptions.destinationURL,
					SubscriptionID: deliveryOptions.subscriptionID,
				})
				if environment.output == "json" {
					if err := environment.writeJSON(report); err != nil {
						return err
					}
				} else {
					for _, delivery := range report.Deliveries {
						fmt.Fprintf(environment.stdout, "step %d.%d\t%s@%d\tHTTP %d\t%s\n", delivery.Step, delivery.Iteration, delivery.Result.EventType, delivery.Result.EventVersion, delivery.Result.Status, delivery.Result.Outcome)
					}
					fmt.Fprintf(environment.stdout, "Workflow %s: %d deliveries\n", entry.ID, len(report.Deliveries))
				}
				if runErr != nil {
					return deliveryError(runErr)
				}
				return nil
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
			options.ScenarioDefinitionID = entry.ID
			options.ScenarioSourceVersion = entry.SourceVersion
			options.Actors = entry.Scenario.Actors
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
