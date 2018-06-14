package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_image",
    "container_push",
)

container_image(
    name = "etcd-manager-base",
    base = "@debian_base_amd64//image",
    directory = "/opt",
    tars = [
        "@etcd_2_2_1_tar//file",  # etcd2 version
        "@etcd_3_1_12_tar//file",  # k8s 1.10
        "@etcd_3_2_18_tar//file",  # current k8s 1.11
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

container_push(
    name = "push-etcd-manager",
    format = "Docker",
    image = ":etcd-manager",
    registry = "{STABLE_DOCKER_REGISTRY}",
    repository = "{STABLE_DOCKER_IMAGE_PREFIX}etcd-manager",
    stamp = True,
    tag = "{STABLE_DOCKER_TAG}",
)

container_image(
    name = "etcd-dump",
    base = "etcd-manager-base",
    entrypoint = ["/etcd-dump"],
    files = [
        "//cmd/etcd-dump",
    ],
)

container_push(
    name = "push-etcd-dump",
    format = "Docker",
    image = ":etcd-dump",
    registry = "{STABLE_DOCKER_REGISTRY}",
    repository = "{STABLE_DOCKER_IMAGE_PREFIX}etcd-dump",
    stamp = True,
    tag = "{STABLE_DOCKER_TAG}",
)

container_image(
    name = "etcd-backup",
    base = "etcd-manager-base",
    entrypoint = ["/etcd-backup"],
    files = [
        "//cmd/etcd-backup",
    ],
)

container_push(
    name = "push-etcd-backup",
    format = "Docker",
    image = ":etcd-backup",
    registry = "{STABLE_DOCKER_REGISTRY}",
    repository = "{STABLE_DOCKER_IMAGE_PREFIX}etcd-backup",
    stamp = True,
    tag = "{STABLE_DOCKER_TAG}",
)
