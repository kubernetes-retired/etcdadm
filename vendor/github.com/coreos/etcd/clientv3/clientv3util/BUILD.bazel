load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["util.go"],
    importpath = "github.com/coreos/etcd/clientv3/clientv3util",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/coreos/etcd/clientv3:go_default_library"],
)

go_test(
    name = "go_default_xtest",
    srcs = ["example_key_test.go"],
    importpath = "github.com/coreos/etcd/clientv3/clientv3util_test",
    deps = [
        ":go_default_library",
        "//vendor/github.com/coreos/etcd/clientv3:go_default_library",
    ],
)
