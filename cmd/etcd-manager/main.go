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
	"kope.io/etcd-manager/pkg/etcd"
	"kope.io/etcd-manager/pkg/privateapi"
)

func main() {
	address := "127.0.0.1"
	flag.StringVar(&address, "address", address, "local address to use")

	flag.Parse()

	fmt.Printf("etcd-manager\n")

	uniqueID := privateapi.PeerId(fmt.Sprintf("nodeid-%s", address))

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
	server, err := privateapi.NewServer(myInfo, disco)
	if err != nil {
		glog.Fatalf("error building server: %v", err)
	}

	c := &etcd.EtcdCluster{
		DesiredClusterSize: 3,
		ClusterName:        "etcd-main",

		ClientPort: 4001,
		PeerPort:   2380,
	}
	//me := &etcd.EtcdNode{
	//	Name: "node1",
	//	InternalName: "127.0.0.1",
	//}
	//c.Me = me
	//c.Nodes = append(c.Nodes, me)
	c.ClusterToken = "etcd-cluster-token-" + c.ClusterName

	etcdServer := etcd.NewEtcdServer(server)

	manager, err := etcdServer.NewEtcdManager(c, "/home/justinsb/etcdmanager/data/"+c.ClusterName+"/"+address+"/")
	if err != nil {
		glog.Fatalf("error registering etcd cluster %s to be managed: %v", c.ClusterName, err)
	}
	go manager.Run()

	if err := server.ListenAndServe(grpcAddress); err != nil {
		glog.Fatalf("error creating private API server: %v", err)
	}

	os.Exit(0)
}
