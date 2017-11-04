load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["srv.go"],
    importpath = "github.com/coreos/etcd/pkg/srv",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/coreos/etcd/pkg/types:go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = ["srv_test.go"],
    importpath = "github.com/coreos/etcd/pkg/srv",
    library = ":go_default_library",
    deps = ["//vendor/github.com/coreos/etcd/pkg/testutil:go_default_library"],
)
