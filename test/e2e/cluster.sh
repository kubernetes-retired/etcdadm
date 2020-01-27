#!/usr/bin/env bash
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

set -o errexit
set -o nounset
set -o pipefail

# shellcheck source=/dev/null
source "$(dirname "$0")/utils.sh"

cd_root_path

# Build
make container-build

# Prepare containers
trap "docker rm -f etcdadm-{0,1,2};rm -f ${PWD}/bin/ca.*" EXIT

for ((i=0;i<3;i++))
do
    docker run --name etcdadm-${i} --detach --privileged --security-opt seccomp=unconfined --tmpfs /tmp --tmpfs /run --volume ${PWD}:/etcdadm kindest/node:v1.16.2
done

# Run init
docker exec etcdadm-0 /etcdadm/etcdadm init

# Verify that all endpoints are healthy
docker exec etcdadm-0 /opt/bin/etcdctl.sh endpoint health

# Verify the init container ip address
etcdadm_0_ip=$(docker inspect --format {{.NetworkSettings.Networks.bridge.IPAddress}} etcdadm-0)

# Copy CA certs from etcdadm-0 container to bin directory
docker cp etcdadm-0:/etc/etcd/pki/ca.crt ${PWD}/bin/
docker cp etcdadm-0:/etc/etcd/pki/ca.key ${PWD}/bin/

# Add more members
for ((i=1;i<3;i++))
do
    echo "Copying CA certs to container etcdadm-${i}"
    # Copy CA certs to container
    docker exec etcdadm-${i} mkdir -p /etc/etcd/pki
    docker cp ${PWD}/bin/ca.crt etcdadm-${i}:/etc/etcd/pki/
    docker cp ${PWD}/bin/ca.key etcdadm-${i}:/etc/etcd/pki/

    echo "Joining etcd member etcdadm-${i}"
    docker exec etcdadm-${i} /etcdadm/etcdadm join https://${etcdadm_0_ip}:2379 --name etcdadm-${i}
    docker exec etcdadm-${i} /opt/bin/etcdctl.sh endpoint health --cluster -w table

    sleep 5
done

# Verify that all endpoints are healthy
echo "Etcd cluster members:"
docker exec etcdadm-0 /opt/bin/etcdctl.sh endpoint health --cluster -w table
