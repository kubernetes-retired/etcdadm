/*
Copyright 2019 The Kubernetes Authors.

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

package volumes

import (
	"fmt"
	"time"
)

// Backoff ...
type Backoff struct {
	Duration time.Duration
	Steps    int
}

type conditionFunc func() (done bool, err error)

// SleepUntil is for retrying functions
func SleepUntil(backoff Backoff, condition conditionFunc) error {
	for backoff.Steps > 0 {
		if ok, err := condition(); err != nil || ok {
			return err
		}
		if backoff.Steps == 1 {
			break
		}
		backoff.Steps--
		time.Sleep(backoff.Duration)

	}
	return fmt.Errorf("Timed out waiting for the condition")
}
