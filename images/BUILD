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

container_push(
   name = "push-etcd-manager",
   image = ":etcd-manager",
   format = "Docker",
   registry = "index.docker.io",
   repository = "{DOCKER_REGISTRY}/etcd-manager",
   tag = "{DOCKER_TAG}",
   stamp = True,
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
   image = ":etcd-dump",
   format = "Docker",
   registry = "index.docker.io",
   repository = "{DOCKER_REGISTRY}/etcd-dump",
   tag = "{DOCKER_TAG}",
   stamp = True,
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
   image = ":etcd-backup",
   format = "Docker",
   registry = "index.docker.io",
   repository = "{DOCKER_REGISTRY}/etcd-backup",
   tag = "{DOCKER_TAG}",
   stamp = True,
)