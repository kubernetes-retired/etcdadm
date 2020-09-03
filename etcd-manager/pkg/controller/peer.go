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

package controller

import (
	"context"
	"fmt"

	protoetcd "sigs.k8s.io/etcdadm/etcd-manager/pkg/apis/etcd"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/privateapi"
)

type peer struct {
	Id    privateapi.PeerId
	info  *privateapi.PeerInfo
	peers privateapi.Peers
}

func (p *peer) String() string {
	s := fmt.Sprintf("peer{%s}", p.info)
	return s
}

func (m *EtcdController) newPeer(info *privateapi.PeerInfo) *peer {
	p := &peer{
		Id:    privateapi.PeerId(info.Id),
		info:  info,
		peers: m.peers,
	}
	return p
}

func (p *peer) rpcDoBackup(ctx context.Context, doBackupRequest *protoetcd.DoBackupRequest) (*protoetcd.DoBackupResponse, error) {
	peerGrpcClient, err := p.peers.GetPeerClient(p.Id)
	if err != nil {
		return nil, fmt.Errorf("error getting peer client %q: %v", p.Id, err)
	}
	peerClient := protoetcd.NewEtcdManagerServiceClient(peerGrpcClient)
	return peerClient.DoBackup(ctx, doBackupRequest)
}

func (p *peer) rpcDoRestore(ctx context.Context, doRestoreRequest *protoetcd.DoRestoreRequest) (*protoetcd.DoRestoreResponse, error) {
	peerGrpcClient, err := p.peers.GetPeerClient(p.Id)
	if err != nil {
		return nil, fmt.Errorf("error getting peer client %q: %v", p.Id, err)
	}
	peerClient := protoetcd.NewEtcdManagerServiceClient(peerGrpcClient)
	return peerClient.DoRestore(ctx, doRestoreRequest)
}

func (p *peer) rpcJoinCluster(ctx context.Context, joinClusterRequest *protoetcd.JoinClusterRequest) (*protoetcd.JoinClusterResponse, error) {
	peerGrpcClient, err := p.peers.GetPeerClient(p.Id)
	if err != nil {
		return nil, fmt.Errorf("error getting peer client %q: %v", p.Id, err)
	}
	peerClient := protoetcd.NewEtcdManagerServiceClient(peerGrpcClient)
	return peerClient.JoinCluster(ctx, joinClusterRequest)
}

func (p *peer) rpcGetInfo(ctx context.Context, request *protoetcd.GetInfoRequest) (*protoetcd.GetInfoResponse, error) {
	peerGrpcClient, err := p.peers.GetPeerClient(p.Id)
	if err != nil {
		return nil, fmt.Errorf("error getting peer client %q: %v", p.Id, err)
	}
	peerClient := protoetcd.NewEtcdManagerServiceClient(peerGrpcClient)
	return peerClient.GetInfo(ctx, request)
}

func (p *peer) rpcUpdateEndpoints(ctx context.Context, request *protoetcd.UpdateEndpointsRequest) (*protoetcd.UpdateEndpointsResponse, error) {
	peerGrpcClient, err := p.peers.GetPeerClient(p.Id)
	if err != nil {
		return nil, fmt.Errorf("error getting peer client %q: %v", p.Id, err)
	}
	peerClient := protoetcd.NewEtcdManagerServiceClient(peerGrpcClient)
	return peerClient.UpdateEndpoints(ctx, request)
}

func (p *peer) rpcReconfigure(ctx context.Context, request *protoetcd.ReconfigureRequest) (*protoetcd.ReconfigureResponse, error) {
	peerGrpcClient, err := p.peers.GetPeerClient(p.Id)
	if err != nil {
		return nil, fmt.Errorf("error getting peer client %q: %v", p.Id, err)
	}
	peerClient := protoetcd.NewEtcdManagerServiceClient(peerGrpcClient)
	return peerClient.Reconfigure(ctx, request)
}

func (p *peer) rpcStopEtcd(ctx context.Context, request *protoetcd.StopEtcdRequest) (*protoetcd.StopEtcdResponse, error) {
	peerGrpcClient, err := p.peers.GetPeerClient(p.Id)
	if err != nil {
		return nil, fmt.Errorf("error getting peer client %q: %v", p.Id, err)
	}
	peerClient := protoetcd.NewEtcdManagerServiceClient(peerGrpcClient)
	return peerClient.StopEtcd(ctx, request)
}
