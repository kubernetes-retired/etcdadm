BAZEL_FLAGS=--features=pure --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64
.PHONY: all
all: test

.PHONY: test
test:
	bazel test //... --test_output=streamed

.PHONY: stress-test
stress-test:
	bazel test //... --test_output=streamed --runs_per_test=10

.PHONY: gofmt
gofmt:
	gofmt -w -s cmd/ pkg/

.PHONY: goimports
goimports:
	goimports -w cmd/ pkg/ test/

.PHONY: push-etcd-manager
push-etcd-manager:
	bazel run ${BAZEL_FLAGS} //images:push-etcd-manager

.PHONY: push-etcd-dump
push-etcd-dump:
	bazel run ${BAZEL_FLAGS} //images:push-etcd-dump

.PHONY: push-etcd-backup
push-etcd-backup:
	bazel run ${BAZEL_FLAGS} //images:push-etcd-backup

.PHONY: push
push: push-etcd-manager push-etcd-dump push-etcd-backup
	echo "pushed images"

.PHONY: gazelle
gazelle:
	bazel run //:gazelle -- fix
	git checkout -- vendor
	rm -f vendor/github.com/coreos/etcd/cmd/etcd
	#rm vendor/github.com/golang/protobuf/protoc-gen-go/testdata/multi/BUILD.bazel

.PHONY: dep-ensure
dep-ensure:
	echo "'make dep-ensure' has been replaced by 'make vendor'"
	exit 1

.PHONY: vendor
vendor:
	go mod vendor
	find vendor/ -name "BUILD" -delete
	find vendor/ -name "BUILD.bazel" -delete
	bazel run //:gazelle

.PHONY: staticcheck-all
staticcheck-all:
	go list ./... | xargs go run honnef.co/go/tools/cmd/staticcheck

# staticcheck-working is the subset of packages that we have cleaned up
# We gradually want to sync up staticcheck-all with staticcheck-working
.PHONY: staticcheck-working
staticcheck-working:
	go list ./... | grep -v "etcd-manager/pkg/[cepv]" | xargs go run honnef.co/go/tools/cmd/staticcheck
