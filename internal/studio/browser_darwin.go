//go:build darwin

package studio

import "os/exec"

func openBrowser(target string) error {
	return exec.Command("open", target).Start()
}
