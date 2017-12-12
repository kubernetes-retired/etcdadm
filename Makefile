DOCKER_REGISTRY?=$(shell whoami)
DOCKER_TAG?=latest

.PHONY: all
all: test

.PHONY: test
test:
	bazel test //test/... --test_output=streamed

.PHONY: gofmt
gofmt:
	gofmt -w -s cmd/ pkg/

.PHONY: goimports
goimports:
	goimports -w cmd/ pkg/ test/


.PHONY: image-etcd-manager
image-etcd-manager:
	bazel run //images:etcd-manager
	docker tag bazel/images:etcd-manager ${DOCKER_REGISTRY}/etcd-manager:${DOCKER_TAG}

.PHONY: push-etcd-manager
push-etcd-manager: image-etcd-manager
	docker push ${DOCKER_REGISTRY}/etcd-manager:${DOCKER_TAG}

.PHONY: push
push: push-etcd-manager
	echo "pushed image"
