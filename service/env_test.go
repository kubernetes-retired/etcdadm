/*
Copyright 2021 The Kubernetes Authors.

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

package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/etcdadm/apis"
	"sigs.k8s.io/yaml"
)

func TestBuildEnv(t *testing.T) {
	basedir := "testdata/buildenvironment"
	dirs, err := os.ReadDir(basedir)
	if err != nil {
		t.Fatalf("failed to read directory %q: %v", basedir, err)
	}
	for _, f := range dirs {
		dir := filepath.Join(basedir, f.Name())

		if !f.IsDir() {
			t.Errorf("expected directory %s", dir)
			continue
		}

		t.Run(f.Name(), func(t *testing.T) {
			testBuildEnvDir(t, dir)
		})
	}
}

func testBuildEnvDir(t *testing.T, dir string) {
	inputPath := filepath.Join(dir, "in.yaml")
	inputBytes, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("failed to read file %q: %v", inputPath, err)
	}
	cfg := &apis.EtcdAdmConfig{}
	if err := yaml.Unmarshal(inputBytes, cfg); err != nil {
		t.Fatalf("failed to parse file %q: %v", inputPath, err)
	}

	got, err := BuildEnvironment(cfg)
	if err != nil {
		t.Fatalf("BuildEnvironment failed: %v", err)
	}

	wantPath := filepath.Join(dir, "want.txt")
	checkGolden(t, wantPath, got)
}

func checkGolden(t *testing.T, wantPath string, got []byte) {
	updateGoldenOutput := os.Getenv("UPDATE_GOLDEN_OUTPUT") != ""

	want, err := os.ReadFile(wantPath)
	if err != nil {
		if os.IsNotExist(err) && updateGoldenOutput {
			// ignore
		} else {
			t.Fatalf("failed to read file %q: %v", wantPath, err)
		}
	}

	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("unexpected result; diff %s", diff)
	}

	if updateGoldenOutput {
		if err := os.WriteFile(wantPath, got, 0644); err != nil {
			t.Errorf("failed to write file %q: %v", wantPath, err)
		}
	}
}
