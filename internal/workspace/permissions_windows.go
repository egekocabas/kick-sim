//go:build windows

package workspace

import (
	"fmt"
	"os"
)

// ValidatePrivateKeyPermissions confirms the key exists; Windows ACLs require manual verification.
func ValidatePrivateKeyPermissions(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("inspect private key: %w", err)
	}
	return nil
}

// PrivateKeyProtectionWarning explains the manual ACL check required on Windows.
func PrivateKeyProtectionWarning() string {
	return "the private-key ACL could not be verified automatically; restrict it to the current Windows user before sharing this workspace"
}
