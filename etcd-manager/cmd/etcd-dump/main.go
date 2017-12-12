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

	//address := "127.0.0.1"
	//flag.StringVar(&address, "address", address, "local address to use")
	//memberCount := 1
	//flag.IntVar(&memberCount, "members", memberCount, "initial cluster size; cluster won't start until we have a quorum of this size")
	//clusterName := ""
	//flag.StringVar(&clusterName, "cluster-name", clusterName, "name of cluster")
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
