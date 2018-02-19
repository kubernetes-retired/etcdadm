#!/bin/bash -e

pushd pkg/privateapi/
ln -sf ../../bazel-bin/pkg/privateapi/privateapi_go_proto~/kope.io/etcd-manager/pkg/privateapi/cluster.pb.go
popd

pushd pkg/apis/etcd/
ln -sf ../../../bazel-bin/./pkg/apis/etcd/etcd_go_proto~/kope.io/etcd-manager/pkg/apis/etcd/etcdapi.pb.go
popd
