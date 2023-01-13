#!/usr/bin/env bash
# Copyright 2019 The Kubernetes Authors.
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
export GO111MODULE=on
go build

# Prepare container
trap "docker rm -f etcdadm-0" EXIT
docker run --name etcdadm-0 --detach --privileged --security-opt seccomp=unconfined --tmpfs /tmp --tmpfs /run  --volume ${PWD}:/etcdadm kindest/node:v1.26.0

# Run init
docker exec etcdadm-0  /etcdadm/etcdadm init

# Verify that all endpoints are healthy
docker exec etcdadm-0 /opt/bin/etcdctl.sh endpoint health
