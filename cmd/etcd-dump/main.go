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
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"os"

	"kope.io/etcd-manager/pkg/etcd"
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

	if out == "" {
		fmt.Printf("out is required\n")
		os.Exit(1)
	}

	f, err := os.Create(out)
	if err != nil {
		fmt.Printf("unable to create file %q: %v\n", out, err)
		os.Exit(1)
	}
	defer f.Close()

	gzw := gzip.NewWriter(f)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	if err := etcd.DumpBackup(datadir, tw); err != nil {
		fmt.Printf("error during dump: %v\n", err)
		os.Exit(1)
	}
}
