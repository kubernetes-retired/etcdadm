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
	"os"

	"github.com/golang/glog"
	apis_etcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/backup"
	"kope.io/etcd-manager/pkg/controller"
	"kope.io/etcd-manager/pkg/etcd"
	"kope.io/etcd-manager/pkg/privateapi"
)

func main() {
	address := "127.0.0.1"
	flag.StringVar(&address, "address", address, "local address to use")
	clusterName := ""
	flag.StringVar(&clusterName, "cluster-name", clusterName, "name of cluster")
	backupStorePath := ""
	flag.StringVar(&backupStorePath, "backup-store", backupStorePath, "backup store location")

	flag.Parse()

	fmt.Printf("etcd-manager\n")

	if clusterName == "" {
		fmt.Fprintf(os.Stderr, "cluster-name is required\n")
		os.Exit(1)
	}

	glog.Warningf("hard coding backup store location")
	backupStorePath = "file:///home/justinsb/etcdmanager/backups/" + clusterName + "/"
	if backupStorePath == "" {
		fmt.Fprintf(os.Stderr, "backup-store is required\n")
		os.Exit(1)
	}

	baseDir := "/home/justinsb/etcdmanager/data/" + clusterName + "/" + address + "/"
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		glog.Fatalf("error doing mkdirs on base directory %s: %v", baseDir, err)
	}

	uniqueID, err := privateapi.PersistentPeerId(baseDir)
	if err != nil {
		glog.Fatalf("error getting persistent peer id: %v", err)
	}

	grpcPort := 8000
	discoMe := privateapi.DiscoveryNode{
		ID: uniqueID,
	}
	discoMe.Addresses = append(discoMe.Addresses, privateapi.DiscoveryAddress{
		IP: fmt.Sprintf("%s:%d", address, grpcPort),
	})
	disco, err := privateapi.NewFilesystemDiscovery("/tmp/discovery", discoMe)
	if err != nil {
		glog.Fatalf("error building discovery: %v", err)
	}

	grpcAddress := fmt.Sprintf("%s:%d", address, grpcPort)
	myInfo := privateapi.PeerInfo{
		Id:        string(uniqueID),
		Addresses: []string{address},
	}
	peerServer, err := privateapi.NewServer(myInfo, disco)
	if err != nil {
		glog.Fatalf("error building server: %v", err)
	}

	//c := &apis_etcd.EtcdNode{
	//	DesiredClusterSize: 3,
	//	ClusterName:        "etcd-main",
	//
	//	ClientPort: 4001,
	//	PeerPort:   2380,
	//}
	var clientUrls []string
	clientPort := 4001
	clientUrls = append(clientUrls, fmt.Sprintf("http://%s:%d", address, clientPort))

	var peerUrls []string
	peerPort := 2380
	peerUrls = append(peerUrls, fmt.Sprintf("http://%s:%d", address, peerPort))

	me := &apis_etcd.EtcdNode{
		Name:       string(uniqueID),
		ClientUrls: clientUrls,
		PeerUrls:   peerUrls,
	}
	//c.Me = me
	//c.Nodes = append(c.Nodes, me)
	//c.ClusterToken = "etcd-cluster-token-" + c.ClusterName

	backupStore, err := backup.NewStore(backupStorePath)
	if err != nil {
		glog.Fatalf("error initializing backup store: %v", err)
	}

	etcdServer := etcd.NewEtcdServer(baseDir, clusterName, me, peerServer)

	go etcdServer.Run()

	c, err := controller.NewEtcdController(backupStore, clusterName, peerServer)
	if err != nil {
		glog.Fatalf("error building etcd controller: %v", err)
	}
	go c.Run()

	if err := peerServer.ListenAndServe(grpcAddress); err != nil {
		glog.Fatalf("error creating private API server: %v", err)
	}

	os.Exit(0)
}
