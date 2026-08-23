//go:build windows

package workspace

import (
	"fmt"
	"os"
)

func ValidatePrivateKeyPermissions(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("inspect private key: %w", err)
	}
	return nil
}

func PrivateKeyProtectionWarning() string {
	return "the private-key ACL could not be verified automatically; restrict it to the current Windows user before sharing this workspace"
}
