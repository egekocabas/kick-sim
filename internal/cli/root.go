package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/egekocabas/kick-sim/internal/signing"
	"github.com/egekocabas/kick-sim/internal/simulator"
	"github.com/egekocabas/kick-sim/internal/workspace"
	"github.com/spf13/cobra"
)

func Execute() error {
	return NewRootCommand(os.Stdout, os.Stderr).Execute()
}

func NewRootCommand(stdout, stderr io.Writer) *cobra.Command {
	var workspacePath string

	root := &cobra.Command{
		Use:           "kick-sim",
		Short:         "Send locally signed Kick webhook events",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().StringVar(&workspacePath, "workspace", ".kick-sim", "workspace directory")

	root.AddCommand(newInitCommand(stdout, &workspacePath))
	root.AddCommand(newEventCommand(stdout, &workspacePath))

	return root
}

func newInitCommand(stdout io.Writer, workspacePath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create a simulator workspace and RSA key pair",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			paths, err := workspace.Init(*workspacePath)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintf(stdout, "Workspace created: %s\nPublic key: %s\n", paths.Root, paths.PublicKey)
			return err
		},
	}
}

func newEventCommand(stdout io.Writer, workspacePath *string) *cobra.Command {
	event := &cobra.Command{
		Use:   "event",
		Short: "Generate and deliver webhook events",
	}
	event.AddCommand(newTriggerCommand(stdout, workspacePath))
	return event
}

func newTriggerCommand(stdout io.Writer, workspacePath *string) *cobra.Command {
	var destinationURL string
	var content string
	var sender string
	var senderID int64
	var broadcaster string
	var broadcasterID int64

	command := &cobra.Command{
		Use:   "trigger <event-type>",
		Short: "Deliver a signed chat.message.sent webhook",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 || args[0] != simulator.ChatMessageSentType {
				return fmt.Errorf("expected event type %q", simulator.ChatMessageSentType)
			}
			return nil
		},
		RunE: func(command *cobra.Command, _ []string) error {
			privateKey, err := signing.ReadPrivateKey(workspace.PathsFor(*workspacePath).PrivateKey)
			if err != nil {
				return fmt.Errorf("load simulator private key: %w", err)
			}

			result, err := simulator.TriggerChatMessage(command.Context(), privateKey, simulator.ChatMessageOptions{
				DestinationURL: destinationURL,
				Content:        content,
				Sender: simulator.UserInput{
					UserID:   senderID,
					Username: sender,
				},
				Broadcaster: simulator.UserInput{
					UserID:     broadcasterID,
					Username:   broadcaster,
					IsVerified: true,
				},
				Timeout: 10 * time.Second,
			})
			if err != nil {
				return err
			}

			_, err = fmt.Fprintf(stdout,
				"Delivered %s@1\nStatus: %d\nDuration: %s\nMessage ID: %s\nSubscription ID: %s\n",
				simulator.ChatMessageSentType,
				result.StatusCode,
				result.Duration.Round(time.Millisecond),
				result.MessageID,
				result.SubscriptionID,
			)
			return err
		},
	}

	command.Flags().StringVar(&destinationURL, "destination-url", "", "loopback webhook URL")
	command.Flags().StringVar(&content, "content", "Hello from Kick Sim", "chat message content")
	command.Flags().StringVar(&sender, "sender", "viewer_42", "sender username")
	command.Flags().Int64Var(&senderID, "sender-id", 987654321, "sender user ID")
	command.Flags().StringVar(&broadcaster, "broadcaster", "broadcaster", "broadcaster username")
	command.Flags().Int64Var(&broadcasterID, "broadcaster-id", 123456789, "broadcaster user ID")
	_ = command.MarkFlagRequired("destination-url")

	return command
}

func executeForTest(ctx context.Context, command *cobra.Command, args ...string) error {
	command.SetArgs(args)
	return command.ExecuteContext(ctx)
}
