# Copyright (c) 2018 The etcdadm authors
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
#
# Usage:
# make                 # builds the artifact
# make container-build # build artifact on a Linux based container using golang:1.16

SHELL := /usr/bin/env bash
CWD := $(shell pwd)
BIN := etcdadm
PACKAGE_GOPATH := /go/src/sigs.k8s.io/$(BIN)
LDFLAGS := $(shell source ./version.sh ; KUBE_ROOT=. ; KUBE_GIT_VERSION=${VERSION_OVERRIDE} ; kube::version::ldflags)
GIT_STORAGE_MOUNT := $(shell source ./git_utils.sh; container_git_storage_mount)
GO_IMAGE ?= golang:1.16

.PHONY: clean container-build default ensure diagrams $(BIN)

default: $(BIN)

container-build:
	docker run --rm -e VERSION_OVERRIDE=${VERSION_OVERRIDE}  -e GOPROXY -v $(PWD):$(PACKAGE_GOPATH) -w $(PACKAGE_GOPATH) $(GIT_STORAGE_MOUNT) ${GO_IMAGE} /bin/bash -c "make"

$(BIN):
	GO111MODULE=on go build -ldflags "$(LDFLAGS)"

clean:
	rm -f $(BIN) plantuml.jar

diagrams: plantuml.jar
	java -jar plantuml.jar docs/diagrams/*.md

plantuml.jar:
	wget -O plantuml.jar http://sourceforge.net/projects/plantuml/files/plantuml.1.2019.6.jar/download
