load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    visibility = ["//visibility:private"],
    deps = [
        "//pkg/controller:go_default_library",
        "//pkg/etcd:go_default_library",
        "//pkg/privateapi:go_default_library",
        "@com_github_golang_glog//:go_default_library",
    ],
)

go_binary(
    name = "etcd-manager",
    library = ":go_default_library",
    visibility = ["//visibility:public"],
)
