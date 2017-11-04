load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "backend.go",
        "batch_tx.go",
        "doc.go",
        "metrics.go",
        "read_tx.go",
        "tx_buffer.go",
    ] + select({
        "@io_bazel_rules_go//go/platform:darwin_amd64": [
            "config_default.go",
        ],
        "@io_bazel_rules_go//go/platform:linux_amd64": [
            "config_linux.go",
        ],
        "@io_bazel_rules_go//go/platform:windows_amd64": [
            "config_windows.go",
        ],
        "//conditions:default": [],
    }),
    importpath = "github.com/coreos/etcd/mvcc/backend",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/boltdb/bolt:go_default_library",
        "//vendor/github.com/coreos/pkg/capnslog:go_default_library",
        "//vendor/github.com/prometheus/client_golang/prometheus:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "backend_bench_test.go",
        "backend_test.go",
        "batch_tx_test.go",
    ],
    importpath = "github.com/coreos/etcd/mvcc/backend",
    library = ":go_default_library",
    deps = ["//vendor/github.com/boltdb/bolt:go_default_library"],
)
