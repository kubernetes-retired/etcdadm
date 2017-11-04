DOCKER_REGISTRY?=kopeio
DOCKER_TAG=1.0.20170421

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

.PHONY: push
push: images
	docker push ${DOCKER_REGISTRY}/etcd-manager:${DOCKER_TAG}

.PHONY: images
images:
	bazel run //images:etcd-manager ${DOCKER_REGISTRY}/etcd-manager:${DOCKER_TAG}
