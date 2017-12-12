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
    cmd = ["/etcd-manager"],
    files = [
        "//cmd/etcd-manager",
    ],
)
