#!/bin/bash
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

SHA=`crane manifest gcr.io/distroless/base-debian10:latest-amd64 | sha256sum | cut -f 1 -d ' '`
(bazel query --noshow_progress @distroless-base-amd64//image --output build | grep ${SHA} > /dev/null 2>&1) || (echo "distroless-base-amd64 should be ${SHA}" && exit 1)

SHA=`crane manifest gcr.io/distroless/base-debian10:debug-amd64 | sha256sum | cut -f 1 -d ' '`
(bazel query --noshow_progress @distroless-base-amd64-debug//image --output build | grep ${SHA} > /dev/null 2>&1) || (echo "distroless-base-amd64-debug should be ${SHA}" && exit 1)

SHA=`crane manifest gcr.io/distroless/base-debian10:latest-arm64 | sha256sum | cut -f 1 -d ' '`
(bazel query --noshow_progress @distroless-base-arm64//image --output build | grep ${SHA} > /dev/null 2>&1) || (echo "distroless-base-arm64 should be ${SHA}" && exit 1)

SHA=`crane manifest gcr.io/distroless/base-debian10:debug-arm64 | sha256sum | cut -f 1 -d ' '`
(bazel query --noshow_progress @distroless-base-arm64-debug//image --output build | grep ${SHA} > /dev/null 2>&1) || (echo "distroless-base-arm64-debug should be ${SHA}" && exit 1)

echo "Verified!"
