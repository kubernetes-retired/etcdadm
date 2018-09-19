/**
 *   Copyright 2018 Platform9 Systems, Inc.
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
	"os"
	"os/exec"
)

// Exists checks whether the file or directory exists.
func Exists(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CopyFile copies file from src to dest
func CopyFile(srcFile, destFile string) error {
	if err := exec.Command("cp", "-f", srcFile, destFile).Run(); err != nil {
		return err
	}
	return nil
}
