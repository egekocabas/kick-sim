package cli

import (
	"fmt"
	"os"

	"github.com/egekocabas/kick-sim/internal/config"
	"github.com/egekocabas/kick-sim/internal/signing"
	"github.com/egekocabas/kick-sim/internal/workspace"
	"github.com/spf13/cobra"
)

func newKeysCommand(environment *environment) *cobra.Command {
	command := &cobra.Command{Use: "keys", Short: "Manage the simulator signing key pair"}
	command.AddCommand(newKeysInitCommand(environment))
	command.AddCommand(newKeysPublicCommand(environment))
	command.AddCommand(newKeysInfoCommand(environment))
	command.AddCommand(newKeysRotateCommand(environment))
	return command
}

func newKeysInitCommand(environment *environment) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Generate a simulator key pair",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := environment.resolve(false)
			if err != nil {
				return err
			}
			if err := workspace.InitKeys(root); err != nil {
				return workspaceError(err)
			}
			paths := workspace.PathsFor(root)
			if environment.output == "json" {
				return environment.writeJSON(map[string]string{
					"publicKey": paths.PublicKey,
					"warning":   workspace.PrivateKeyProtectionWarning(),
				})
			}
			if warning := workspace.PrivateKeyProtectionWarning(); warning != "" {
				fmt.Fprintf(environment.stderr, "Warning: %s\n", warning)
			}
			_, err = fmt.Fprintf(environment.stdout, "Simulator key pair created\nPublic key: %s\n", paths.PublicKey)
			return err
		},
	}
}

func newKeysPublicCommand(environment *environment) *cobra.Command {
	var pathOnly bool
	command := &cobra.Command{
		Use:   "public",
		Short: "Print the simulator public key",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := environment.resolve(false)
			if err != nil {
				return err
			}
			configuration, err := config.Load(workspace.PathsFor(root).Config)
			if err != nil {
				return workspaceError(err)
			}
			path := config.ResolvePath(root, configuration.Signing.PublicKey)
			if pathOnly {
				if environment.output == "json" {
					return environment.writeJSON(map[string]string{"path": path})
				}
				_, err = fmt.Fprintln(environment.stdout, path)
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return workspaceError(fmt.Errorf("read public key: %w", err))
			}
			if environment.output == "json" {
				return environment.writeJSON(map[string]string{"path": path, "pem": string(data)})
			}
			_, err = environment.stdout.Write(data)
			return err
		},
	}
	command.Flags().BoolVar(&pathOnly, "path", false, "print only the public-key path")
	return command
}

func newKeysInfoCommand(environment *environment) *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show simulator key information",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := environment.resolve(false)
			if err != nil {
				return err
			}
			configuration, err := config.Load(workspace.PathsFor(root).Config)
			if err != nil {
				return workspaceError(err)
			}
			publicPath := config.ResolvePath(root, configuration.Signing.PublicKey)
			privatePath := config.ResolvePath(root, configuration.Signing.PrivateKey)
			publicKey, err := signing.ReadPublicKey(publicPath)
			if err != nil {
				return workspaceError(err)
			}
			fingerprint, err := signing.Fingerprint(publicKey)
			if err != nil {
				return err
			}
			info, err := os.Stat(publicPath)
			if err != nil {
				return workspaceError(err)
			}
			privateKey, privateError := signing.ReadPrivateKey(privatePath)
			matchingPrivateKey := privateError == nil && privateKey.PublicKey.N.Cmp(publicKey.N) == 0
			result := map[string]any{
				"path":               publicPath,
				"algorithm":          "RSA",
				"bits":               publicKey.N.BitLen(),
				"fingerprint":        fingerprint,
				"modifiedAt":         info.ModTime(),
				"matchingPrivateKey": matchingPrivateKey,
			}
			if environment.output == "json" {
				return environment.writeJSON(result)
			}
			_, err = fmt.Fprintf(environment.stdout,
				"Path: %s\nAlgorithm: RSA\nBits: %d\nFingerprint: %s\nModified: %s\nPrivate key present: %t\n",
				publicPath,
				publicKey.N.BitLen(),
				fingerprint,
				info.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
				matchingPrivateKey,
			)
			return err
		},
	}
}

func newKeysRotateCommand(environment *environment) *cobra.Command {
	return &cobra.Command{
		Use:   "rotate",
		Short: "Replace the simulator key pair",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := environment.resolve(false)
			if err != nil {
				return err
			}
			if err := workspace.RotateKeys(root); err != nil {
				return workspaceError(err)
			}
			if environment.output == "json" {
				return environment.writeJSON(map[string]bool{"rotated": true})
			}
			_, err = fmt.Fprintln(environment.stdout, "Simulator key pair rotated")
			return err
		},
	}
}
