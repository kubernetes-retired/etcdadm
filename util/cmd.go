package util

import (
	"fmt"
	"log"
	"os"
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

// Run runs a given command
func Run(rootDir string, cmdStr string, arg ...string) error {
	if len(rootDir) > 0 {
		currentPath := os.Getenv("PATH")
		os.Setenv("PATH", currentPath+":"+rootDir)
		log.Printf("Updated PATH variable = %s", os.Getenv("PATH"))
		log.Printf("Running command %s %v", cmdStr, arg)
	}
	cmd := exec.Command(cmdStr, arg...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to run command %s with error %v", cmdStr, err)
	}
	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("failed to get output of command %s with error %v", cmdStr, err)
	}
	return nil
}
