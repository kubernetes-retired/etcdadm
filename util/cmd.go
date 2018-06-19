package util

import (
	"os/exec"
	"strings"
)

func CmdOutputContains(cmd *exec.Cmd, expected string) (bool, error) {
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.Contains(string(out), expected), nil
}
