/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package backupcontroller

import (
	"strings"
	"time"
)

type backupNameInfo struct {
	Timestamp time.Time
	Suffix    string
}

func parseBackupNameInfo(name string) *backupNameInfo {
	info := &backupNameInfo{}

	timeString := name
	z := strings.Index(name, "Z-")
	if z != -1 {
		timeString = name[0 : z+1]
		info.Suffix = name[z+2:]
	}
	t, err := time.Parse(time.RFC3339, timeString)
	if err != nil {
		return nil
	}

	info.Timestamp = t
	return info
}
