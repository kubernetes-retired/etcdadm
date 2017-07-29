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
        "json.go",
        "protolib",  # keep
    ],
    visibility = ["//visibility:public"],
    deps = [
        "@com_github_golang_protobuf//jsonpb:go_default_library",
        "@com_github_golang_protobuf//proto:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_x_net//context:go_default_library",
    ],
)

filegroup(
    name = "go_default_library_protos",
    srcs = ["etcdapi.proto"],
    visibility = ["//visibility:public"],
)
