load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["config.go"],
    importpath = "github.com/coreos/etcd/clientv3/yaml",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/coreos/etcd/clientv3:go_default_library",
        "//vendor/github.com/coreos/etcd/pkg/tlsutil:go_default_library",
        "//vendor/github.com/ghodss/yaml:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["config_test.go"],
    importpath = "github.com/coreos/etcd/clientv3/yaml",
    library = ":go_default_library",
    deps = ["//vendor/github.com/ghodss/yaml:go_default_library"],
)
