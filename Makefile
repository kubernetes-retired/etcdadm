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
DETECTED_OS := $(shell uname -s)
DEP_BIN_GIT := https://github.com/golang/dep/releases/download/v0.4.1/dep-$(DETECTED_OS)-amd64
BIN := etcdadm
PACKAGE_GOPATH := /go/src/github.com/platform9/$(BIN)
DEP_BIN := $(CWD)/bin/dep

.PHONY: container-build default ensure

default: $(BIN)

container-build:
	docker run --rm -v $(PWD):$(PACKAGE_GOPATH) -w $(PACKAGE_GOPATH) golang:1.10 make ensure && make

$(DEP_BIN):
ifeq ($(DEP_BIN),$(CWD)/bin/dep)
	echo "Downloading dep from GitHub" &&\
	mkdir -p $(CWD)/bin &&\
	wget $(DEP_BIN_GIT) -O $(DEP_BIN) &&\
	chmod +x $(DEP_BIN)
endif

ensure: $(DEP_BIN)
	echo $(DEP_BIN)
	$(DEP_BIN) ensure -v

$(BIN):
	go build
