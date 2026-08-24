package cli

import (
	"fmt"
	"os"

	kickopenapi "github.com/egekocabas/kick-sim/internal/openapi"
	"github.com/egekocabas/kick-sim/internal/studio"
	"github.com/spf13/cobra"
)

func newStudioCommand(environment *environment) *cobra.Command {
	var address string
	var noOpen bool
	command := &cobra.Command{
		Use:   "studio",
		Short: "Start the embedded local Studio",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			service, err := environment.service()
			if err != nil {
				return err
			}
			if environment.output == "json" {
				return usageError(fmt.Errorf("studio requires human output"))
			}
			return studio.Run(command.Context(), service, studio.Options{
				Address:     address,
				OpenBrowser: !noOpen,
				Output:      environment.stdout,
			})
		},
	}
	command.Flags().StringVar(&address, "address", studio.DefaultAddress, "loopback listen address")
	command.Flags().BoolVar(&noOpen, "no-open", false, "do not open the browser automatically")
	return command
}

func newOpenAPICommand(environment *environment) *cobra.Command {
	var output string
	command := &cobra.Command{
		Use:   "openapi",
		Short: "Write the Studio OpenAPI document",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			data, err := kickopenapi.Generate()
			if err != nil {
				return err
			}
			if output == "-" {
				_, err = environment.stdout.Write(data)
				return err
			}
			if err := os.WriteFile(output, data, 0o644); err != nil {
				return fmt.Errorf("write OpenAPI document: %w", err)
			}
			if environment.output == "json" {
				return environment.writeJSON(map[string]string{"path": output})
			}
			_, err = fmt.Fprintf(environment.stdout, "OpenAPI document written to %s\n", output)
			return err
		},
	}
	command.Flags().StringVarP(&output, "file", "f", "-", "output file, or - for stdout")
	return command
}
