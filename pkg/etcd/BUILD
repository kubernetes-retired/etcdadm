load("@io_bazel_rules_go//go:def.bzl", "go_library")
load("@org_pubref_rules_protobuf//go:rules.bzl", "go_proto_compile")

go_proto_compile(
    name = "protolib",
    protos = [
        "etcdapi.proto",
    ],
    with_grpc = True,
)

go_library(
    name = "go_default_library",
    srcs = [
        "etcdprocess.go",
        "etcdserver.go",
        "manager.go",
        "protolib",  # keep
    ],
    visibility = ["//visibility:public"],
    deps = [
        "//pkg/privateapi:go_default_library",
        "@com_github_golang_glog//:go_default_library",
        "@org_golang_x_net//context:go_default_library",
    ],
)
