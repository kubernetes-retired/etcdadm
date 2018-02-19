package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_image",
)

container_image(
    name = "etcd-manager-base",
    base = "@debian_base_amd64//image",
    directory = "/opt",
    tars = [
        "@etcd_2_2_1_tar//file",
    ],
)

container_image(
    name = "etcd-manager",
    base = "etcd-manager-base",
    entrypoint = ["/etcd-manager"],
    files = [
        "//cmd/etcd-manager",
    ],
)

container_image(
    name = "etcd-dump",
    base = "etcd-manager-base",
    entrypoint = ["/etcd-dump"],
    files = [
        "//cmd/etcd-dump",
    ],
)

container_image(
    name = "etcd-backup",
    base = "etcd-manager-base",
    entrypoint = ["/etcd-backup"],
    files = [
        "//cmd/etcd-backup",
    ],
)
