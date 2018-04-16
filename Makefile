.PHONY: all
all: test

.PHONY: test
test:
	bazel test //test/... --test_output=streamed

.PHONY: stress-test
stress-test:
	bazel test //test/... --test_output=streamed --runs_per_test=10

.PHONY: gofmt
gofmt:
	gofmt -w -s cmd/ pkg/

.PHONY: goimports
goimports:
	goimports -w cmd/ pkg/ test/

.PHONY: push-etcd-manager
push-etcd-manager:
	bazel run //images:push-etcd-manager

.PHONY: push-etcd-dump
push-etcd-dump:
	bazel run //images:push-etcd-dump

.PHONY: push-etcd-backup
push-etcd-backup:
	bazel run //images:push-etcd-backup

.PHONY: push
push: push-etcd-manager push-etcd-dump push-etcd-backup
	echo "pushed images"

.PHONY: gazelle
gazelle:
	bazel run //:gazelle
	git checkout -- vendor
	rm -f vendor/github.com/coreos/etcd/cmd/etcd
	#rm vendor/github.com/golang/protobuf/protoc-gen-go/testdata/multi/BUILD.bazel

.PHONY: dep-ensure
dep-ensure:
	dep ensure -v
	find vendor/ -name "BUILD" -delete
	find vendor/ -name "BUILD.bazel" -delete
	bazel run //:gazelle -- -proto disable
	rm -f vendor/github.com/coreos/etcd/cmd/etcd
