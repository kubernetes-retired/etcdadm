BAZEL?=bazelisk

.PHONY: vendor
vendor:
	go mod vendor
	find vendor/ -name "BUILD" -delete
	find vendor/ -name "BUILD.bazel" -delete
	${BAZEL} run //:gazelle
