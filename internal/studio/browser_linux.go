//go:build linux

package studio

import "os/exec"

func openBrowser(target string) error {
	return exec.Command("xdg-open", target).Start()
}
