# TODO: Move entirely to bazel?
.PHONY: images

DOCKER_REGISTRY?=kopeio
DOCKER_TAG=1.0.20170421

all: images

gofmt:
	gofmt -w -s cmd/
	gofmt -w -s pkg/

goimports:
	goimports -w cmd/
	goimports -w pkg/

push: images
	docker push ${DOCKER_REGISTRY}/etcd-manager:${DOCKER_TAG}

images:
	bazel run //images:etcd-manager ${DOCKER_REGISTRY}/etcd-manager:${DOCKER_TAG}
