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

package pki

import (
	"testing"
	"time"
)

func TestParseHumanDuration(t *testing.T) {
	grid := []struct {
		Value    string
		Expected time.Duration
	}{
		{"10m", 10 * time.Minute},
		{"24h", 24 * time.Hour},
		{"1d", 24 * time.Hour},
		{"10d", 10 * 24 * time.Hour},
		{"365d", 365 * 24 * time.Hour},
		{"1y", 365 * 24 * time.Hour},
		{"10y", 10 * 365 * 24 * time.Hour},
	}

	for _, g := range grid {
		g := g
		t.Run(g.Value, func(t *testing.T) {
			actual, err := ParseHumanDuration(g.Value)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if actual != g.Expected {
				t.Fatalf("unexpected value; expected=%v, actual=%v", g.Expected, actual)
			}
		})
	}
}
