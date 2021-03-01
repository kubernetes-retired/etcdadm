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

TODAY=`date +%Y%m%d`
VERSION="3.0.${TODAY}"

echo "# Run these commands to do a release"
echo "DOCKER_IMAGE_PREFIX=kopeio/ DOCKER_TAG=${VERSION} make push"
echo "DOCKER_IMAGE_PREFIX=kopeio/ DOCKER_TAG=latest make push"
echo "git tag ${VERSION}"
echo "git push --tags ssh://git@github.com/kopeio/etcd-manager"
echo "./dev/build-assets.sh ${VERSION}"
echo "# Finally, create a new release on GitHub attaching binary assets at dist/"
echo "shipbot -config .shipbot.yaml -tag ${VERSION}"
echo "# To compile release notes:"
echo "LAST_RELEASE=\$(cat dev/last-release)"
echo "git log --oneline \${LAST_RELEASE}..${VERSION} | grep Merge.pull | cut -f 5 -d ' ' | tac  > /tmp/prs"
echo "echo '# ${VERSION}' >> docs/releases/3.0.md"
echo "echo '${VERSION}' > dev/last-release"
echo "relnotes  -config .shipbot.yaml  < /tmp/prs >> docs/releases/3.0.md"
