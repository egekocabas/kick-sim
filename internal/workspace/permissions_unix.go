//go:build !windows

package workspace

import (
	"fmt"
	"os"
	"syscall"
)

func ValidatePrivateKeyPermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect private key permissions: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("private key %s is accessible to group or other users; run chmod 600 %s", path, path)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return fmt.Errorf("private key %s is not owned by the current user", path)
	}
	return nil
}

func PrivateKeyProtectionWarning() string { return "" }
