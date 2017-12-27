load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "election_command.go",
        "error.go",
        "global.go",
        "help.go",
        "lease_renewer_command.go",
        "lock_racer_command.go",
        "root.go",
        "watch_command.go",
    ],
    importpath = "github.com/coreos/etcd/tools/functional-tester/etcd-runner/command",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/coreos/etcd/client:go_default_library",
        "//vendor/github.com/coreos/etcd/clientv3:go_default_library",
        "//vendor/github.com/coreos/etcd/clientv3/concurrency:go_default_library",
        "//vendor/github.com/coreos/etcd/pkg/stringutil:go_default_library",
        "//vendor/github.com/coreos/etcd/version:go_default_library",
        "//vendor/github.com/spf13/cobra:go_default_library",
        "//vendor/github.com/spf13/pflag:go_default_library",
        "//vendor/golang.org/x/time/rate:go_default_library",
        "//vendor/google.golang.org/grpc:go_default_library",
        "//vendor/google.golang.org/grpc/codes:go_default_library",
    ],
)
