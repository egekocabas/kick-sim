package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/egekocabas/kick-sim/internal/app"
	"github.com/egekocabas/kick-sim/internal/events"
	"github.com/spf13/cobra"
)

type payloadFlags struct {
	content       string
	sender        string
	senderID      int64
	broadcaster   string
	broadcasterID int64
	setStrings    []string
	setJSON       []string
	unset         []string
}

type deliveryFlags struct {
	destination    string
	destinationURL string
	subscriptionID string
}

func newEventCommand(environment *environment) *cobra.Command {
	command := &cobra.Command{Use: "event", Short: "Generate, validate, and deliver webhook events"}
	command.AddCommand(newEventListCommand(environment))
	command.AddCommand(newEventShowCommand(environment))
	command.AddCommand(newEventGenerateCommand(environment))
	command.AddCommand(newEventTriggerCommand(environment))
	command.AddCommand(newEventValidateCommand(environment))
	return command
}

func newEventListCommand(environment *environment) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List supported event contracts",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			registry, err := events.NewRegistry()
			if err != nil {
				return err
			}
			definitions := registry.List()
			if environment.output == "json" {
				return environment.writeJSON(definitions)
			}
			for _, definition := range definitions {
				fmt.Fprintf(environment.stdout, "%s@%d\t%s\n", definition.Type, definition.Version, definition.Description)
			}
			return nil
		},
	}
}

func newEventShowCommand(environment *environment) *cobra.Command {
	var eventVersion int
	command := &cobra.Command{
		Use:   "show <event-type>",
		Short: "Show an event contract and JSON Schema",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			registry, err := events.NewRegistry()
			if err != nil {
				return err
			}
			definition, err := registry.Get(args[0], eventVersion)
			if err != nil {
				return usageError(err)
			}
			if environment.output == "json" {
				return environment.writeJSON(definition)
			}
			var pretty bytes.Buffer
			if err := json.Indent(&pretty, definition.Schema, "", "  "); err != nil {
				return err
			}
			fmt.Fprintf(environment.stdout, "%s@%d\n%s\n\n%s\n", definition.Type, definition.Version, definition.Description, pretty.String())
			return nil
		},
	}
	command.Flags().IntVar(&eventVersion, "version", events.ChatMessageSentVersion, "event contract version")
	return command
}

func newEventGenerateCommand(environment *environment) *cobra.Command {
	var eventVersion int
	var payload payloadFlags
	command := &cobra.Command{
		Use:   "generate <event-type>",
		Short: "Generate and validate an event payload",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			service, err := environment.service()
			if err != nil {
				return err
			}
			options, err := payload.options(command, args[0], eventVersion, nil, nil)
			if err != nil {
				return usageError(err)
			}
			generated, err := service.GeneratePayload(options)
			if err != nil {
				return usageError(err)
			}
			if environment.output == "json" {
				return environment.writeJSON(generated)
			}
			data, err := events.Marshal(generated, true)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(environment.stdout, string(data))
			return err
		},
	}
	command.Flags().IntVar(&eventVersion, "version", events.ChatMessageSentVersion, "event contract version")
	payload.register(command)
	return command
}

func newEventTriggerCommand(environment *environment) *cobra.Command {
	var eventVersion int
	var payload payloadFlags
	var deliveryOptions deliveryFlags
	command := &cobra.Command{
		Use:   "trigger <event-type>",
		Short: "Deliver a signed webhook event",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			service, err := environment.service()
			if err != nil {
				return err
			}
			options, err := payload.options(command, args[0], eventVersion, nil, nil)
			if err != nil {
				return usageError(err)
			}
			generated, err := service.Generate(options, deliveryOptions.subscriptionID)
			if err != nil {
				return usageError(err)
			}
			result, err := service.Deliver(command.Context(), generated, deliveryOptions.destination, deliveryOptions.destinationURL, nil)
			if err != nil {
				return deliveryError(err)
			}
			return writeRunResult(environment, result)
		},
	}
	command.Flags().IntVar(&eventVersion, "version", events.ChatMessageSentVersion, "event contract version")
	payload.register(command)
	deliveryOptions.register(command)
	return command
}

func newEventValidateCommand(environment *environment) *cobra.Command {
	var eventVersion int
	var file string
	command := &cobra.Command{
		Use:   "validate <event-type>",
		Short: "Validate a JSON event payload",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			var (
				data []byte
				err  error
			)
			if file == "-" {
				data, err = io.ReadAll(io.LimitReader(command.InOrStdin(), 1<<20))
			} else {
				data, err = os.ReadFile(file)
			}
			if err != nil {
				return usageError(fmt.Errorf("read payload: %w", err))
			}
			registry, err := events.NewRegistry()
			if err != nil {
				return err
			}
			if _, err := registry.ValidateJSON(args[0], eventVersion, data); err != nil {
				return usageError(err)
			}
			if environment.output == "json" {
				return environment.writeJSON(map[string]bool{"valid": true})
			}
			_, err = fmt.Fprintln(environment.stdout, "Payload is valid")
			return err
		},
	}
	command.Flags().IntVar(&eventVersion, "version", events.ChatMessageSentVersion, "event contract version")
	command.Flags().StringVarP(&file, "file", "f", "-", "JSON payload file, or - for stdin")
	return command
}

func (flags *payloadFlags) register(command *cobra.Command) {
	command.Flags().StringVar(&flags.content, "content", "", "chat message content")
	command.Flags().StringVar(&flags.sender, "sender", "", "sender username")
	command.Flags().Int64Var(&flags.senderID, "sender-id", 0, "sender user ID")
	command.Flags().StringVar(&flags.broadcaster, "broadcaster", "", "broadcaster username")
	command.Flags().Int64Var(&flags.broadcasterID, "broadcaster-id", 0, "broadcaster user ID")
	command.Flags().StringArrayVar(&flags.setStrings, "set-string", nil, "set a payload value using /payload/path=value")
	command.Flags().StringArrayVar(&flags.setJSON, "set-json", nil, "set a typed JSON payload value using /payload/path=value")
	command.Flags().StringArrayVar(&flags.unset, "unset", nil, "remove a payload value using /payload/path")
}

func (flags payloadFlags) options(command *cobra.Command, eventType string, eventVersion int, scenarioPayload map[string]any, omit []string) (app.PayloadOptions, error) {
	semantic := map[string]any{}
	if command.Flags().Changed("content") {
		semantic["/payload/content"] = flags.content
	}
	if command.Flags().Changed("sender") {
		semantic["/payload/sender/username"] = flags.sender
	}
	if command.Flags().Changed("sender-id") {
		semantic["/payload/sender/user_id"] = flags.senderID
	}
	if command.Flags().Changed("broadcaster") {
		semantic["/payload/broadcaster/username"] = flags.broadcaster
	}
	if command.Flags().Changed("broadcaster-id") {
		semantic["/payload/broadcaster/user_id"] = flags.broadcasterID
	}
	stringsByPointer, err := parseStringAssignments(flags.setStrings)
	if err != nil {
		return app.PayloadOptions{}, err
	}
	jsonByPointer, err := parseJSONAssignments(flags.setJSON)
	if err != nil {
		return app.PayloadOptions{}, err
	}
	return app.PayloadOptions{
		EventType:      eventType,
		EventVersion:   eventVersion,
		Scenario:       scenarioPayload,
		Omit:           omit,
		StringValues:   stringsByPointer,
		JSONValues:     jsonByPointer,
		Unset:          flags.unset,
		SemanticValues: semantic,
	}, nil
}

func (flags *deliveryFlags) register(command *cobra.Command) {
	command.Flags().StringVar(&flags.destination, "destination", "", "configured destination name")
	command.Flags().StringVar(&flags.destinationURL, "destination-url", "", "temporary loopback destination URL")
	command.Flags().StringVar(&flags.subscriptionID, "subscription-id", "", "explicit delivery subscription ULID")
	command.MarkFlagsMutuallyExclusive("destination", "destination-url")
}

func parseStringAssignments(assignments []string) (map[string]string, error) {
	values := make(map[string]string, len(assignments))
	for _, assignment := range assignments {
		pointer, value, found := strings.Cut(assignment, "=")
		if !found || pointer == "" {
			return nil, fmt.Errorf("invalid string assignment %q", assignment)
		}
		if _, duplicate := values[pointer]; duplicate {
			return nil, fmt.Errorf("duplicate string assignment for %s", pointer)
		}
		values[pointer] = value
	}
	return values, nil
}

func parseJSONAssignments(assignments []string) (map[string]any, error) {
	values := make(map[string]any, len(assignments))
	for _, assignment := range assignments {
		pointer, raw, found := strings.Cut(assignment, "=")
		if !found || pointer == "" {
			return nil, fmt.Errorf("invalid JSON assignment %q", assignment)
		}
		if _, duplicate := values[pointer]; duplicate {
			return nil, fmt.Errorf("duplicate JSON assignment for %s", pointer)
		}
		decoder := json.NewDecoder(strings.NewReader(raw))
		decoder.UseNumber()
		var value any
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("parse JSON assignment for %s: %w", pointer, err)
		}
		var extra any
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("JSON assignment for %s must contain one value", pointer)
		}
		values[pointer] = value
	}
	return values, nil
}

func writeRunResult(environment *environment, result app.RunResult) error {
	if environment.output == "json" {
		return environment.writeJSON(result)
	}
	_, err := fmt.Fprintf(environment.stdout,
		"Delivered %s@%d\nStatus: %d\nDuration: %s\nMessage ID: %s\nSubscription ID: %s\n",
		result.EventType,
		result.EventVersion,
		result.Status,
		result.Duration,
		result.MessageID,
		result.SubscriptionID,
	)
	return err
}
