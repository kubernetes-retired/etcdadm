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

package contextutil

import (
	"context"
	"time"

	"k8s.io/klog/v2"
)

func Sleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)

	select {
	case <-ctx.Done():
		return ctx.Err()

	case <-timer.C:
		return nil
	}
}

func Forever(ctx context.Context, interval time.Duration, f func()) {
	for {
		if ctx.Err() != nil {
			klog.Infof("context cancelled; exiting loop")
			return
		}

		f()

		Sleep(ctx, interval)
	}

}
