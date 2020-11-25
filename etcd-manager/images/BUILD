package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_image",
    "container_layer",
    "container_push",
)

# Layer for etcd 2.2.1, the version of etcd2 that was originally recommended
container_layer(
    name = "etcd-2.2.1-layer-amd64",
    directory = "/opt/etcd-v2.2.1-linux-amd64/",
    files = [
        "@etcd_v2_2_1_source//:etcd",
        "@etcd_v2_2_1_source//etcdctl",
    ],
)

load("etcd.bzl", "supported_etcd_arch_and_version")

# Layer for etcd 3.1.12, as used in k8s 1.10
# Layer for etcd 3.2.18, as originally used in k8s 1.11
# Layer for etcd 3.2.24, updated recommendation for k8s 1.11 and later
# Layer for etcd 3.3.10, updated recommendation for k8s 1.14 and later
# Layer for etcd 3.3.13, updated recommendation for k8s 1.14 and later
# Layer for etcd 3.3.17, updated recommendation for k8s 1.16.3 and later
# Layer for etcd 3.4.3, updated recommendation for k8s 1.17 and later
# Layer for etcd 3.4.13, updated recommendation for k8s 1.19 and later
[
    container_layer(
        name = "etcd-{version}-layer-{arch}".format(
            arch = arch,
            version = version,
        ),
        directory = "/opt/etcd-v{version}-linux-{arch}/".format(
            arch = arch,
            version = version,
        ),
        files = [
            "//:etcd-v{version}-linux-{arch}_etcdctl".format(
                arch = arch,
                version = version,
            ),
            "//:etcd-v{version}-linux-{arch}_etcd".format(
                arch = arch,
                version = version,
            ),
        ],
    )
    for (arch, version) in supported_etcd_arch_and_version()
]

# We build a base layer that includes all the versions of etcd we want to ship
# On non-amd64 we exclude the older versions of etcd
container_image(
    name = "etcd-manager-base",
    base = select({
        "@io_bazel_rules_go//go/platform:amd64": "//images/base:base_image_amd64",
        "@io_bazel_rules_go//go/platform:arm64": "//images/base:base_image_arm64",
    }),
    directory = "/opt",
    layers = select({
        "@io_bazel_rules_go//go/platform:amd64": [
            "etcd-2.2.1-layer-amd64",
            "etcd-3.1.12-layer-amd64",
            "etcd-3.2.18-layer-amd64",
            "etcd-3.2.24-layer-amd64",
            "etcd-3.3.10-layer-amd64",
            "etcd-3.3.13-layer-amd64",
            "etcd-3.3.17-layer-amd64",
            "etcd-3.4.3-layer-amd64",
            "etcd-3.4.13-layer-amd64",
        ],
        "@io_bazel_rules_go//go/platform:arm64": [
            "etcd-3.2.18-layer-arm64",
            "etcd-3.2.24-layer-arm64",
            "etcd-3.3.10-layer-arm64",
            "etcd-3.3.13-layer-arm64",
            "etcd-3.3.17-layer-arm64",
            "etcd-3.4.3-layer-arm64",
            "etcd-3.4.13-layer-arm64",
        ],
    }),
)

container_image(
    name = "etcd-manager",
    architecture = select({
        "@io_bazel_rules_go//go/platform:amd64": "amd64",
        "@io_bazel_rules_go//go/platform:arm64": "arm64",
    }),
    base = "etcd-manager-base",
    entrypoint = ["/etcd-manager"],
    env = select({
        "@io_bazel_rules_go//go/platform:amd64": {},
        "@io_bazel_rules_go//go/platform:arm64": {"ETCD_UNSUPPORTED_ARCH": "arm64"},
    }),
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
    tag = select({
        "@io_bazel_rules_go//go/platform:amd64": "{STABLE_DOCKER_TAG}-amd64",
        "@io_bazel_rules_go//go/platform:arm64": "{STABLE_DOCKER_TAG}-arm64",
    }),
)

container_image(
    name = "etcd-dump",
    architecture = select({
        "@io_bazel_rules_go//go/platform:amd64": "amd64",
        "@io_bazel_rules_go//go/platform:arm64": "arm64",
    }),
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
    tag = select({
        "@io_bazel_rules_go//go/platform:amd64": "{STABLE_DOCKER_TAG}-amd64",
        "@io_bazel_rules_go//go/platform:arm64": "{STABLE_DOCKER_TAG}-arm64",
    }),
)

container_image(
    name = "etcd-backup",
    architecture = select({
        "@io_bazel_rules_go//go/platform:amd64": "amd64",
        "@io_bazel_rules_go//go/platform:arm64": "arm64",
    }),
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
    tag = select({
        "@io_bazel_rules_go//go/platform:amd64": "{STABLE_DOCKER_TAG}-amd64",
        "@io_bazel_rules_go//go/platform:arm64": "{STABLE_DOCKER_TAG}-arm64",
    }),
)
