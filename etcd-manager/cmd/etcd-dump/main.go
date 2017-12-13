/*
Copyright 2017 The Kubernetes Authors.

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

package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"kope.io/etcd-manager/pkg/etcd/dump"
)

func main() {
	flag.Set("logtostderr", "true")

	datadir := ""
	flag.StringVar(&datadir, "data-dir", datadir, "data dir location")
	out := ""
	flag.StringVar(&out, "out", out, "output file")

	flag.Parse()

	fmt.Printf("etcd-dump\n")

	if datadir == "" {
		fmt.Printf("data-dir is required\n")
		os.Exit(1)
	}

	_, err := os.Stat(datadir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("data-dir %q not found", datadir)
			os.Exit(1)
		} else {
			fmt.Printf("error checking for data-dir %q: %v", datadir, err)
			os.Exit(1)
		}
	}

	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		fmt.Printf("error creating temp dir: %v", err)
		os.Exit(1)
	}

	defer func() {
		if err := os.RemoveAll(tmpdir); err != nil {
			glog.Warningf("error removing tmpdir %q: %v", tmpdir, err)
		}
	}()

	if err := copyTree(datadir, tmpdir); err != nil {
		fmt.Printf("error copying to temp dir: %v", err)
		os.Exit(1)
	}

	var listener dump.DumpSink
	if out == "" {
		listener, err = dump.NewStreamDumpSink(os.Stdout)
		if err != nil {
			fmt.Printf("unable to create stream: %v\n", err)
			os.Exit(1)
		}
	} else {
		listener, err = dump.NewTarDumpSink(out)
		if err != nil {
			fmt.Printf("unable to create file %q: %v\n", out, err)
			os.Exit(1)
		}
	}

	if err := dump.DumpBackup(tmpdir, listener); err != nil {
		fmt.Printf("error during dump: %v\n", err)
		os.Exit(1)
	}
}

func copyTree(srcdir string, destdir string) error {
	files, err := ioutil.ReadDir(srcdir)
	if err != nil {
		return fmt.Errorf("error reading dir %q: %v", srcdir, err)
	}

	for _, f := range files {
		src := filepath.Join(srcdir, f.Name())
		dest := filepath.Join(destdir, f.Name())

		if f.IsDir() {
			if err := os.Mkdir(dest, 0755); err != nil {
				return fmt.Errorf("error creating dir %q: %v", dest, err)
			}
			if err := copyTree(src, dest); err != nil {
				return err
			}
		} else {
			if err := copyFile(src, dest); err != nil {
				return err
			}
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}
