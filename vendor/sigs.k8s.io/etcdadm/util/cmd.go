/**
 *   Copyright 2018 The etcdadm authors
 *
 *   Licensed under the Apache License, Version 2.0 (the "License");
 *   you may not use this file except in compliance with the License.
 *   You may obtain a copy of the License at
 *
 *       http://www.apache.org/licenses/LICENSE-2.0
 *
 *   Unless required by applicable law or agreed to in writing, software
 *   distributed under the License is distributed on an "AS IS" BASIS,
 *   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *   See the License for the specific language governing permissions and
 *   limitations under the License.
 */

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
