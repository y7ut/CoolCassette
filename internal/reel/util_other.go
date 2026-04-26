//go:build !windows

package reel

import "os/exec"

func hideWindow(c *exec.Cmd) {}
