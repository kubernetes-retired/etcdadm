# Copyright (c) 2018 Platform9 Systems, Inc.
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
# make ensure          # runs dep ensure
# make container-build # build artifact on a Linux based container using golang 1.10

SHELL := /usr/bin/env bash
CWD := $(shell pwd)
BIN := etcdadm
PACKAGE_GOPATH := /go/src/github.com/platform9/$(BIN)
LDFLAGS := $(shell source ./version.sh ; KUBE_ROOT=. ; kube::version::ldflags)
GIT_STORAGE_MOUNT := $(shell source ./git_utils.sh; container_git_storage_mount)

.PHONY: clean container-build default ensure

default: $(BIN)

container-build:
	docker run --rm -v $(PWD):$(PACKAGE_GOPATH) -w $(PACKAGE_GOPATH) $(GIT_STORAGE_MOUNT) golang:1.10 /bin/bash -c "make ensure && make"

$(BIN):
	go build -ldflags "$(LDFLAGS)"

clean:
	rm -f $(BIN)
