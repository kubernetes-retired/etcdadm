#!/bin/bash
# Copyright 2018 The Kubernetes Authors.
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

set -x
NAME=$(basename $(git remote get-url origin | sed 's/\.git//'))
GITHUB_USER=$(basename $(dirname $(git remote get-url origin | sed 's/\.git//')))
TAG=$(git tag --points-at HEAD )

GO111MODULE=off go get github.com/aktau/github-release
which upx > /dev/null  || (sudo apt-get update && sudo apt-get install -y upx-ucl)
make clean $NAME
upx $NAME
github-release release --user $GITHUB_USER --repo $NAME --tag $TAG
github-release upload --user $GITHUB_USER --repo $NAME --tag $TAG --name $NAME --file $NAME
