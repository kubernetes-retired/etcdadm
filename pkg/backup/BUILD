load("@io_bazel_rules_go//go:def.bzl", "go_library")
load("@org_pubref_rules_protobuf//go:rules.bzl", "go_proto_compile")

go_library(
    name = "go_default_library",
    srcs = ["store.go"],
    visibility = ["//visibility:public"],
    deps = [
        "//pkg/apis/etcd:go_default_library",
        "@com_github_golang_glog//:go_default_library",
    ],
)
