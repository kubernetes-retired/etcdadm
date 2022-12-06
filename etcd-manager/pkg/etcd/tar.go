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

package etcd

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"

	"k8s.io/klog/v2"
)

type gzFile struct {
	File string
}

// gunzip will uncompress the file srcFile, writing it to destFile
func (g *gzFile) expand(destFile string) error {
	src, err := os.Open(g.File)
	if err != nil {
		return fmt.Errorf("error opening %q: %v", g.File, err)
	}
	defer src.Close()

	dest, err := os.Create(destFile)
	if err != nil {
		return fmt.Errorf("error opening %q: %v", g.File, err)
	}
	defer dest.Close()

	gz, err := gzip.NewReader(src)
	if err != nil {
		return fmt.Errorf("error reading gzip %q (source corrupted?): %v", g.File, err)
	}

	n, err := io.Copy(dest, gz)
	if err != nil {
		return fmt.Errorf("error expanding file: %v", err)
	}
	klog.V(2).Infof("expanded snapshot file, size=%d bytes", n)

	if err := gz.Close(); err != nil {
		return fmt.Errorf("error completing file expansion: %v", err)
	}

	return nil
}
