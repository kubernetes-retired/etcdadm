package util

import (
	"fmt"
	"os/exec"
	"strings"
)

// CmdOutputContains run a given given and looks for the expected result in the commands
// output
func CmdOutputContains(cmd *exec.Cmd, expected string) (bool, error) {
	out, err := cmd.Output()
	if err != nil {
		switch v := err.(type) {
		case *exec.Error:
			return false, fmt.Errorf("failed to run command %q: %s", v.Name, v.Err)
		case *exec.ExitError:
			return false, fmt.Errorf("command %q failed: %q", cmd.Path, v.Stderr)
		default:
			return false, err
		}
	}
	return strings.Contains(string(out), expected), nil
}
