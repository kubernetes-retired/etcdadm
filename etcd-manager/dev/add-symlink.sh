#!/bin/bash -e
# Copyright 2020 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

pushd pkg/privateapi/
ln -sf ../../bazel-bin/pkg/privateapi/linux_amd64_stripped/privateapi_go_proto~/sigs.k8s.io/etcdadm/etcd-manager/pkg/privateapi/cluster.pb.go
popd

pushd pkg/apis/etcd/
ln -sf ../../../bazel-bin/pkg/apis/etcd/linux_amd64_stripped/etcd_go_proto~/sigs.k8s.io/etcdadm/etcd-manager/pkg/apis/etcd/etcdapi.pb.go
popd
