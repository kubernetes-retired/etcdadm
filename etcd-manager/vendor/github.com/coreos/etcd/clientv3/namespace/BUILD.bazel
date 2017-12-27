load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "kv.go",
        "lease.go",
        "util.go",
        "watch.go",
    ],
    importpath = "github.com/coreos/etcd/clientv3/namespace",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/coreos/etcd/clientv3:go_default_library",
        "//vendor/github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes:go_default_library",
        "//vendor/github.com/coreos/etcd/etcdserver/etcdserverpb:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["util_test.go"],
    importpath = "github.com/coreos/etcd/clientv3/namespace",
    library = ":go_default_library",
)
