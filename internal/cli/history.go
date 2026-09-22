package cli

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/egekocabas/kick-sim/internal/history"
	"github.com/egekocabas/kick-sim/internal/signing"
	"github.com/spf13/cobra"
)

func newHistoryCommand(environment *environment) *cobra.Command {
	command := &cobra.Command{Use: "history", Short: "Inspect and replay retained deliveries"}
	command.AddCommand(newHistoryListCommand(environment))
	command.AddCommand(newHistoryShowCommand(environment))
	command.AddCommand(newHistoryReplayCommand(environment))
	command.AddCommand(newHistoryDeleteCommand(environment))
	return command
}

func newHistoryListCommand(environment *environment) *cobra.Command {
	var limit int
	var offset int
	command := &cobra.Command{
		Use:   "list",
		Short: "List chronological delivery activity",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			service, err := environment.service()
			if err != nil {
				return err
			}
			store, err := service.History()
			if err != nil {
				return workspaceError(err)
			}
			defer store.Close()
			items, err := store.ListActivity(command.Context(), limit, offset)
			if err != nil {
				return workspaceError(err)
			}
			if environment.output == "json" {
				return environment.writeJSON(items)
			}
			if len(items) == 0 {
				_, err = fmt.Fprintln(environment.stdout, "No retained delivery activity")
				return err
			}
			table := newTable(environment.stdout, "ATTEMPT", "EVENT", "STATUS", "OUTCOME", "DURATION")
			for _, item := range items {
				status := "-"
				if item.Status != 0 {
					status = fmt.Sprint(item.Status)
				}
				fmt.Fprintf(table, "%s\t%s@%d\t%s\t%s\t%.2fms\n", item.AttemptID, item.EventType, item.EventVersion, status, item.Outcome, item.DurationMS)
			}
			return table.Flush()
		},
	}
	command.Flags().IntVar(&limit, "limit", 50, "maximum activity rows")
	command.Flags().IntVar(&offset, "offset", 0, "activity row offset")
	return command
}

func newHistoryShowCommand(environment *environment) *cobra.Command {
	return &cobra.Command{
		Use:   "show <attempt-id>",
		Short: "Inspect a retained delivery attempt",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			detail, err := historyAttempt(command, environment, args[0])
			if err != nil {
				return err
			}
			if environment.output == "json" {
				return environment.writeJSON(detail)
			}
			status := "— (no HTTP response)"
			if detail.Attempt.ResponseStatus != 0 {
				status = fmt.Sprintf("%d %s", detail.Attempt.ResponseStatus, http.StatusText(detail.Attempt.ResponseStatus))
			}
			fmt.Fprintf(environment.stdout,
				"Attempt: %s\nEvent: %s@%d\nOutcome: %s\nStatus: %s\nDestination: %s\nDuration: %.2fms\n\nHeaders:\n",
				detail.Attempt.ID, detail.Event.EventType, detail.Event.EventVersion,
				detail.Attempt.Outcome, status,
				detail.Attempt.URL, detail.Attempt.DurationMS,
			)
			keys := make([]string, 0, len(detail.Attempt.RequestHeaders))
			for key := range detail.Attempt.RequestHeaders {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				fmt.Fprintf(environment.stdout, "%s: %s\n", key, detail.Attempt.RequestHeaders[key])
			}
			fmt.Fprintf(environment.stdout, "\nSignature input:\n%s\n\nRaw body:\n%s\n\nResponse:\n",
				signing.SignatureInput(detail.Attempt.RequestHeaders["Kick-Event-Message-Id"], detail.Attempt.RequestHeaders["Kick-Event-Message-Timestamp"], []byte(detail.Event.RawBody)),
				detail.Event.RawBody,
			)
			if detail.Attempt.Error != "" {
				fmt.Fprintf(environment.stdout, "Error: %s\n", detail.Attempt.Error)
			}
			if detail.Attempt.ResponseStatus == 0 {
				fmt.Fprintln(environment.stdout, "No HTTP response received. Check that the receiver is running at the destination URL.")
				return nil
			}
			fmt.Fprintf(environment.stdout, "HTTP %s\n", status)
			keys = keys[:0]
			for key := range detail.Attempt.ResponseHeaders {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				for _, value := range detail.Attempt.ResponseHeaders[key] {
					fmt.Fprintf(environment.stdout, "%s: %s\n", key, value)
				}
			}
			fmt.Fprintln(environment.stdout, "\nResponse body:")
			switch {
			case detail.Attempt.ResponseBody != "":
				fmt.Fprintln(environment.stdout, detail.Attempt.ResponseBody)
			case detail.Attempt.ResponseStatus == http.StatusNoContent:
				fmt.Fprintln(environment.stdout, "(empty — HTTP 204 No Content)")
			default:
				fmt.Fprintln(environment.stdout, "(empty)")
			}
			if detail.Attempt.ResponseTruncated {
				fmt.Fprintln(environment.stdout, "(response body truncated)")
			}
			return nil
		},
	}
}

func newHistoryReplayCommand(environment *environment) *cobra.Command {
	var mode string
	command := &cobra.Command{
		Use:   "replay <attempt-id>",
		Short: "Replay a retained delivery attempt",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if mode != "exact" && mode != "regenerated" {
				return usageError(fmt.Errorf("replay mode must be exact or regenerated"))
			}
			service, err := environment.service()
			if err != nil {
				return err
			}
			result, replayErr := service.Replay(command.Context(), args[0], mode)
			if replayErr != nil {
				return deliveryError(replayErr)
			}
			return writeRunResult(environment, result)
		},
	}
	command.Flags().StringVar(&mode, "mode", "exact", "replay mode: exact or regenerated")
	return command
}

func newHistoryDeleteCommand(environment *environment) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <run-id>",
		Short: "Delete a retained run and its delivery attempts",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			service, err := environment.service()
			if err != nil {
				return err
			}
			store, err := service.History()
			if err != nil {
				return workspaceError(err)
			}
			defer store.Close()
			if err := store.DeleteRun(command.Context(), args[0]); err != nil {
				return workspaceError(err)
			}
			if environment.output == "json" {
				return environment.writeJSON(map[string]any{"deleted": true, "runId": args[0]})
			}
			_, err = fmt.Fprintf(environment.stdout, "Deleted retained run %s\n", args[0])
			return err
		},
	}
}

func historyAttempt(command *cobra.Command, environment *environment, id string) (history.AttemptDetail, error) {
	service, err := environment.service()
	if err != nil {
		return history.AttemptDetail{}, err
	}
	store, err := service.History()
	if err != nil {
		return history.AttemptDetail{}, workspaceError(err)
	}
	defer store.Close()
	detail, err := store.GetAttempt(command.Context(), id)
	if err != nil {
		return history.AttemptDetail{}, workspaceError(err)
	}
	return detail, nil
}
