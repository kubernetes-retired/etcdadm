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

set -o errexit -o nounset -o pipefail
set -x;

# cd to the repo root
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "${REPO_ROOT}"

if [[ -z "${VERSION:-}" ]]; then
  VERSION=$(git describe --always)
fi

if [[ -z "${DOCKER_REGISTRY:-}" ]]; then
  DOCKER_REGISTRY=gcr.io
fi

if [[ -z "${DOCKER_IMAGE_PREFIX:-}" ]]; then
  DOCKER_IMAGE_PREFIX=k8s-staging-etcdadm/
fi

if [[ -z "${ARTIFACT_LOCATION:-}" ]]; then
  echo "must set ARTIFACT_LOCATION for binary artifacts"
  exit 1
fi

# Make sure ARTIFACT_LOCATION ends in a slash
if [[ "${ARTIFACT_LOCATION}" != */ ]]; then
  ARTIFACT_LOCATION="${ARTIFACT_LOCATION}/"
fi

if [[ -n "${INSTALL_BAZELISK:-}" ]]; then
  DOWNLOAD_URL="https://github.com/bazelbuild/bazelisk/releases/download/v1.7.2/bazelisk-linux-amd64"
  echo "Downloading bazelisk from $DOWNLOAD_URL"
  curl -L --output "/tmp/bazelisk" "${DOWNLOAD_URL}"
  chmod +x "/tmp/bazelisk"
  # Install to /usr/local/bin, as bazel
  mv "/tmp/bazelisk" "/usr/local/bin/bazel"
fi

# Build and upload etcdadm binary
make etcdadm
gsutil -h "Cache-Control:private, max-age=0, no-transform" -m cp -n etcdadm ${ARTIFACT_LOCATION}${VERSION}/etcdadm

# Build and upload etcd-manager images & binaries
DOCKER_REGISTRY=${DOCKER_REGISTRY} DOCKER_IMAGE_PREFIX=${DOCKER_IMAGE_PREFIX} DOCKER_TAG=${VERSION} make -C etcd-manager push
./etcd-manager/dev/build-assets.sh ${VERSION}
gsutil -h "Cache-Control:private, max-age=0, no-transform" -m cp -n etcd-manager/dist/ ${ARTIFACT_LOCATION}${VERSION}/etcd-manager/
