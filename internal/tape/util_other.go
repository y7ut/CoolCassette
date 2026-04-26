//go:build !windows

package tape

import "os/exec"

func hideWindow(c *exec.Cmd) {}
