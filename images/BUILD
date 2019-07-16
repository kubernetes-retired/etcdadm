package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_image",
    "container_layer",
    "container_push",
)

# Layer for etcd 2.2.1, the version of etcd2 that was recommended
container_layer(
    name = "etcd-2-2-1-layer",
    directory = "/opt/etcd-v2.2.1-linux-amd64/",
    files = [
        "@etcd_v2_2_1_source//:etcd",
        "@etcd_v2_2_1_source//etcdctl",
    ],
)

#container_layer(
#    name = "etcd-2-2-1-layer-upstream",
#    directory = "/opt/",
#    tars = [
#        "@etcd_2_2_1_tar//file",
#],
#)

# Layer for etcd 3.1.12, as used in k8s 1.10
container_layer(
    name = "etcd-3-1-12-layer",
    directory = "/opt/etcd-v3.1.12-linux-amd64/",
    files = [
        "//:etcd-v3.1.12-linux-amd64_etcdctl",
        "//:etcd-v3.1.12-linux-amd64_etcd",
    ],
)

# Layer for etcd 3.2.18, as originally used in k8s 1.11
container_layer(
    name = "etcd-3-2-18-layer",
    directory = "/opt/etcd-v3.2.18-linux-amd64/",
    files = [
        "//:etcd-v3.2.18-linux-amd64_etcdctl",
        "//:etcd-v3.2.18-linux-amd64_etcd",
    ],
)

# Layer for etcd 3.2.24, updated recommendation for k8s 1.11 and later
container_layer(
    name = "etcd-3-2-24-layer",
    directory = "/opt/etcd-v3.2.24-linux-amd64/",
    files = [
        "//:etcd-v3.2.24-linux-amd64_etcdctl",
        "//:etcd-v3.2.24-linux-amd64_etcd",
    ],
)

# Layer for etcd 3.3.10, updated recommendation for k8s 1.14 and later
container_layer(
    name = "etcd-3-3-10-layer",
    directory = "/opt/etcd-v3.3.10-linux-amd64/",
    files = [
        "//:etcd-v3.3.10-linux-amd64_etcdctl",
        "//:etcd-v3.3.10-linux-amd64_etcd",
    ],
)

container_image(
    name = "etcd-manager-base",
    base = "@debian_base_amd64//image",
    directory = "/opt",
    layers = [
        "etcd-2-2-1-layer",
        "etcd-3-1-12-layer",
        "etcd-3-2-18-layer",
        "etcd-3-2-24-layer",
        "etcd-3-3-10-layer",
    ],
)

container_image(
    name = "etcd-manager",
    base = "etcd-manager-base",
    entrypoint = ["/etcd-manager"],
    files = [
        "//cmd/etcd-manager",
        "//cmd/etcd-manager-ctl",
    ],
)

container_push(
    name = "push-etcd-manager",
    format = "Docker",
    image = ":etcd-manager",
    registry = "{STABLE_DOCKER_REGISTRY}",
    repository = "{STABLE_DOCKER_IMAGE_PREFIX}etcd-manager",
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
    tag = "{STABLE_DOCKER_TAG}",
)

container_image(
    name = "etcd-backup",
    base = "etcd-manager-base",
    entrypoint = ["/etcd-backup"],
    files = [
        "//cmd/etcd-backup",
        "//cmd/etcd-backup-ctl",
    ],
)

container_push(
    name = "push-etcd-backup",
    format = "Docker",
    image = ":etcd-backup",
    registry = "{STABLE_DOCKER_REGISTRY}",
    repository = "{STABLE_DOCKER_IMAGE_PREFIX}etcd-backup",
    tag = "{STABLE_DOCKER_TAG}",
)
