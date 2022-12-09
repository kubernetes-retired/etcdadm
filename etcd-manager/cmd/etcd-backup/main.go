/*
Copyright 2018 The Kubernetes Authors.

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
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"os"
	"time"

	"k8s.io/klog/v2"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/backup"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/backupcontroller"
)

func main() {
	klog.InitFlags(nil)

	clusterName := ""
	flag.StringVar(&clusterName, "cluster-name", clusterName, "name of cluster")
	backupStorePath := "/backups"
	flag.StringVar(&backupStorePath, "backup-store", backupStorePath, "backup store location")
	dataDir := "/data"
	flag.StringVar(&dataDir, "data-dir", dataDir, "directory for storing etcd data")
	clientURL := "http://127.0.0.1:4001"
	flag.StringVar(&clientURL, "client-url", clientURL, "URL on which to connect to etcd")
	interval := "15m"
	flag.StringVar(&interval, "interval", interval, "backup frequency")
	clientCAFile := ""
	flag.StringVar(&clientCAFile, "client-ca-file", clientCAFile, "path to the ca certificate")
	clientCertFile := ""
	flag.StringVar(&clientCertFile, "client-cert-file", clientCertFile, "path to the client tls certificate")
	clientKeyFile := ""
	flag.StringVar(&clientKeyFile, "client-key-file", clientKeyFile, "path to the client tls cert key")

	flag.Parse()

	fmt.Printf("etcd-backup agent\n")

	if clusterName == "" {
		fmt.Fprintf(os.Stderr, "cluster-name is required\n")
		os.Exit(1)
	}

	if backupStorePath == "" {
		fmt.Fprintf(os.Stderr, "backup-store is required\n")
		os.Exit(1)
	}

	backupInterval, err := time.ParseDuration(interval)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot parse interval %q", interval)
		os.Exit(1)
	}

	ctx := context.TODO()

	var etcdClientTLSConfig *tls.Config
	if (clientCertFile != "") && (clientKeyFile != "") && (clientCAFile != "") {
		cert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
		if err != nil {
			klog.Fatalf("error creating keypair from provided etcd certificate and key files: %v", err)
			os.Exit(1)
		}

		raw, err := os.ReadFile(clientCAFile)
		if err != nil {
			klog.Fatalf("error loading etcd ca cert file: %v", err)
			os.Exit(1)
		}
		roots := x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(raw)
		if !ok {
			klog.Fatalf("error parsing etcd ca cert file")
			os.Exit(1)
		}

		etcdClientTLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}, RootCAs: roots}
	}

	backupStore, err := backup.NewStore(backupStorePath)
	if err != nil {
		klog.Fatalf("error initializing backup store: %v", err)
	}
	clientURLs := []string{clientURL}
	c, err := backupcontroller.NewBackupController(backupStore, clusterName, clientURLs, etcdClientTLSConfig, dataDir, backupInterval)
	if err != nil {
		klog.Fatalf("error building backup controller: %v", err)
	}

	c.Run(ctx)

	os.Exit(0)
}
