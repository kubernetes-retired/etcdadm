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
	"time"

	"github.com/golang/glog"
)

// Backoff ...
type Backoff struct {
	Duration time.Duration
	Attempts int
}

type conditionFunc func() (done bool, err error)

// SleepUntil is for retrying functions
// It calls condition repeatedly, with backoff.
// When condition returns done==true or a err != nil; this function returns those values.
// If first we exhaust the backoff Attempts; we return false, nil
func SleepUntil(backoff Backoff, condition conditionFunc) (done bool, err error) {
	for backoff.Attempts > 0 {
		if done, err := condition(); err != nil || done {
			return done, err
		}
		backoff.Attempts--
		if backoff.Attempts == 0 {
			break
		}
		if err != nil {
			glog.Infof("retrying after error: %v", err)
		}
		time.Sleep(backoff.Duration)
	}
	return false, nil
}
