package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/egekocabas/kick-sim/internal/compatibility"
	"github.com/egekocabas/kick-sim/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCommand(environment *environment) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show build version information",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			info := version.Current()
			if environment.output == "json" {
				return environment.writeJSON(info)
			}
			_, err := fmt.Fprintf(environment.stdout, "kick-sim %s (%s, %s)\n", info.Version, info.Commit, info.Date)
			return err
		},
	}
}

func newCompatibilityCommand(environment *environment) *cobra.Command {
	return &cobra.Command{
		Use:   "compatibility",
		Short: "Show supported event contracts and upstream provenance",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			metadata, err := compatibility.Load()
			if err != nil {
				return err
			}
			if environment.output == "json" {
				return environment.writeJSON(struct {
					Version string `json:"kickSimVersion"`
					compatibility.Metadata
				}{Version: version.Version, Metadata: metadata})
			}
			fmt.Fprintf(environment.stdout,
				"Kick Sim: %s\nKick docs revision: %s\nRetrieved: %s\nBundle: %s\n\nSupported events:\n",
				version.Version,
				metadata.Upstream.Commit,
				metadata.Upstream.RetrievedAt,
				metadata.BundleDigest,
			)
			names := make([]string, 0, len(metadata.SupportedEvents))
			for name := range metadata.SupportedEvents {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				versions := metadata.SupportedEvents[name]
				parts := make([]string, len(versions))
				for index, eventVersion := range versions {
					parts[index] = fmt.Sprint(eventVersion)
				}
				fmt.Fprintf(environment.stdout, "  %s@%s\n", name, strings.Join(parts, ","))
			}
			fmt.Fprintln(environment.stdout, "\nKnown differences:")
			for _, difference := range metadata.KnownDifferences {
				fmt.Fprintf(environment.stdout, "  - %s\n", difference)
			}
			return nil
		},
	}
}
