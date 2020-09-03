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

package dump

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	etcdclient "kope.io/etcd-manager/pkg/etcdclient"
)

type TarDumpSink struct {
	tw  *tar.Writer
	gzw *gzip.Writer
	f   *os.File

	// doneDirs helps us synthesize directories
	doneDirs map[string]bool
}

var _ etcdclient.NodeSink = &TarDumpSink{}

func NewTarDumpSink(p string) (*TarDumpSink, error) {
	f, err := os.Create(p)
	if err != nil {
		return nil, fmt.Errorf("unable to create file %q: %v", p, err)
	}
	s := &TarDumpSink{
		doneDirs: make(map[string]bool),
		f:        f,
	}

	s.gzw = gzip.NewWriter(f)

	s.tw = tar.NewWriter(s.gzw)

	return s, nil
}

func (s *TarDumpSink) Put(ctx context.Context, key string, value []byte) error {
	name := key

	if err := s.ensureDirs(path.Dir(name)); err != nil {
		return err
	}

	hdr := &tar.Header{
		Name:     strings.TrimPrefix(name, "/"),
		Mode:     0644,
		Typeflag: tar.TypeReg,
		Size:     int64(len(value)),
	}

	// write the header
	if err := s.tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("error writing tar header: %v", err)
	}

	if len(value) != 0 {
		_, err := s.tw.Write(value)
		if err != nil {
			return fmt.Errorf("error writing tar data: %v", err)
		}
	}

	return nil
}

func (s *TarDumpSink) ensureDirs(p string) error {
	if p == "" || p == "/" {
		return nil
	}

	if s.doneDirs[p] {
		return nil
	}

	if err := s.ensureDirs(path.Dir(p)); err != nil {
		return err
	}

	name := p
	if !strings.HasSuffix(p, "/") {
		name += "/"
	}

	hdr := &tar.Header{
		Name:     name,
		Mode:     0755,
		Typeflag: tar.TypeDir,
	}

	hdr.Name = strings.TrimPrefix(hdr.Name, "/")

	// write the header
	if err := s.tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("error writing tar header: %v", err)
	}

	s.doneDirs[p] = true

	return nil
}

func (s *TarDumpSink) Close() error {
	var err error

	errC := s.tw.Close()
	if errC != nil {
		err = fmt.Errorf("error closing tar: %v", errC)
	}

	errC = s.gzw.Close()
	if errC != nil {
		err = fmt.Errorf("error closing gzip: %v", errC)
	}

	errC = s.f.Close()
	if errC != nil {
		err = fmt.Errorf("error closing file: %v", errC)
	}

	return err
}
