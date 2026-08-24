package cli

import (
	"fmt"
	"os"

	"github.com/egekocabas/kick-sim/internal/config"
	"github.com/egekocabas/kick-sim/internal/workspace"
	"github.com/spf13/cobra"
)

func newInitCommand(environment *environment) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create a simulator workspace and RSA key pair",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := environment.resolve(true)
			if err != nil {
				return err
			}
			paths, err := workspace.Init(root)
			if err != nil {
				return workspaceError(err)
			}
			if environment.output == "json" {
				return environment.writeJSON(map[string]string{
					"workspace": paths.Root,
					"publicKey": paths.PublicKey,
					"warning":   workspace.PrivateKeyProtectionWarning(),
				})
			}
			if warning := workspace.PrivateKeyProtectionWarning(); warning != "" {
				fmt.Fprintf(environment.stderr, "Warning: %s\n", warning)
			}
			_, err = fmt.Fprintf(environment.stdout,
				"Kick Sim workspace created:\n  %s\n\nSimulator public key:\n  %s\n\nStart the local Studio:\n  kick-sim studio\n",
				paths.Root,
				paths.PublicKey,
			)
			return err
		},
	}
}

func newWorkspaceCommand(environment *environment) *cobra.Command {
	command := &cobra.Command{Use: "workspace", Short: "Inspect and validate the active workspace"}
	command.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print the resolved workspace path",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := environment.resolve(false)
			if err != nil {
				return err
			}
			if environment.output == "json" {
				return environment.writeJSON(map[string]string{"workspace": root})
			}
			_, err = fmt.Fprintln(environment.stdout, root)
			return err
		},
	})
	command.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate configuration and signing keys",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := environment.resolve(false)
			if err != nil {
				return err
			}
			if err := workspace.Validate(root); err != nil {
				return workspaceError(err)
			}
			if environment.output == "json" {
				return environment.writeJSON(map[string]any{"workspace": root, "valid": true})
			}
			_, err = fmt.Fprintln(environment.stdout, "Workspace is valid")
			return err
		},
	})
	command.AddCommand(&cobra.Command{
		Use:   "info",
		Short: "Show workspace paths and configuration summary",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := environment.resolve(false)
			if err != nil {
				return err
			}
			paths := workspace.PathsFor(root)
			configuration, err := config.Load(paths.Config)
			if err != nil {
				return workspaceError(err)
			}
			info := map[string]any{
				"workspace":          root,
				"config":             paths.Config,
				"scenarios":          paths.Scenarios,
				"defaultDestination": configuration.DefaultDestination,
				"formatVersion":      configuration.Version,
			}
			if environment.output == "json" {
				return environment.writeJSON(info)
			}
			_, err = fmt.Fprintf(environment.stdout,
				"Workspace: %s\nConfig: %s\nScenarios: %s\nFormat version: %d\nDefault destination: %s\n",
				root,
				paths.Config,
				paths.Scenarios,
				configuration.Version,
				configuration.DefaultDestination,
			)
			return err
		},
	})
	return command
}

func newConfigCommand(environment *environment) *cobra.Command {
	command := &cobra.Command{Use: "config", Short: "Inspect and validate workspace configuration"}
	command.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show the active configuration",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := environment.resolve(false)
			if err != nil {
				return err
			}
			path := workspace.PathsFor(root).Config
			value, err := config.Load(path)
			if err != nil {
				return workspaceError(err)
			}
			if environment.output == "json" {
				return environment.writeJSON(value)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return workspaceError(err)
			}
			_, err = environment.stdout.Write(data)
			return err
		},
	})
	command.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate the active configuration",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := environment.resolve(false)
			if err != nil {
				return err
			}
			if _, err := config.Load(workspace.PathsFor(root).Config); err != nil {
				return workspaceError(err)
			}
			if environment.output == "json" {
				return environment.writeJSON(map[string]bool{"valid": true})
			}
			_, err = fmt.Fprintln(environment.stdout, "Configuration is valid")
			return err
		},
	})
	return command
}
