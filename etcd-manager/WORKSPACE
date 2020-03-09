load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")

http_archive(
    name = "io_bazel_rules_go",
    urls = [
        "https://storage.googleapis.com/bazel-mirror/github.com/bazelbuild/rules_go/releases/download/v0.20.2/rules_go-v0.20.2.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.20.2/rules_go-v0.20.2.tar.gz",
    ],
    sha256 = "b9aa86ec08a292b97ec4591cf578e020b35f98e12173bbd4a921f84f583aebd9",
)

http_archive(
    name = "bazel_gazelle",
    urls = [
        "https://storage.googleapis.com/bazel-mirror/github.com/bazelbuild/bazel-gazelle/releases/download/v0.19.1/bazel-gazelle-v0.19.1.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.19.1/bazel-gazelle-v0.19.1.tar.gz",
    ],
    sha256 = "86c6d481b3f7aedc1d60c1c211c6f76da282ae197c3b3160f54bd3a8f847896f",
)

load("@io_bazel_rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_toolchains")

go_rules_dependencies()

go_register_toolchains(
    go_version = "1.13.4",
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")

gazelle_dependencies()

#=============================================================================

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "14ac30773fdb393ddec90e158c9ec7ebb3f8a4fd533ec2abbfd8789ad81a284b",
    strip_prefix = "rules_docker-0.12.1",
    urls = ["https://github.com/bazelbuild/rules_docker/releases/download/v0.12.1/rules_docker-v0.12.1.tar.gz"],
)

load(
    "@io_bazel_rules_docker//repositories:repositories.bzl",
    container_repositories = "repositories",
)

container_repositories()

load("@io_bazel_rules_docker//repositories:deps.bzl", container_deps = "deps")

container_deps()

# Note: We can't (easily) use distroless because we need: fsck, blkid, mount, others? to mount disks
# We also have to use debian-hyperkube-base because we need nsenter / fsck

load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_pull",
)

container_pull(
    name = "debian-hyperkube-base-amd64",
    architecture = "amd64",
    digest = "sha256:5d4ea2fb5fbe9a9a9da74f67cf2faefc881968bc39f2ac5d62d9167e575812a1",
    registry = "k8s.gcr.io",
    repository = "debian-hyperkube-base",
    tag = "0.12.1",  # ignored, but kept here for documentation
)

#=============================================================================
# etcd rules
http_file(
    name = "etcd_2_2_1_tar",
    sha256 = "59f7985c81b6bc551246c165c2fd83e33a063875e4e0c61920b1d90a4910f462",
    urls = ["https://github.com/coreos/etcd/releases/download/v2.2.1/etcd-v2.2.1-linux-amd64.tar.gz"],
)

http_file(
    name = "etcd_3_1_12_tar",
    sha256 = "4b22184bef1bba8b4908b14bae6af4a6d33ec2b91e4f7a240780e07fa43f2111",
    urls = ["https://github.com/coreos/etcd/releases/download/v3.1.12/etcd-v3.1.12-linux-amd64.tar.gz"],
)

http_file(
    name = "etcd_3_2_18_tar",
    sha256 = "b729db0732448064271ea6fdcb901773c4fe917763ca07776f22d0e5e0bd4097",
    urls = ["https://github.com/coreos/etcd/releases/download/v3.2.18/etcd-v3.2.18-linux-amd64.tar.gz"],
)

http_file(
    name = "etcd_3_2_24_tar",
    sha256 = "947849dbcfa13927c81236fb76a7c01d587bbab42ab1e807184cd91b026ebed7",
    urls = ["https://github.com/coreos/etcd/releases/download/v3.2.24/etcd-v3.2.24-linux-amd64.tar.gz"],
)

http_file(
    name = "etcd_3_3_10_tar",
    sha256 = "1620a59150ec0a0124a65540e23891243feb2d9a628092fb1edcc23974724a45",
    urls = ["https://github.com/coreos/etcd/releases/download/v3.3.10/etcd-v3.3.10-linux-amd64.tar.gz"],
)

http_file(
    name = "etcd_3_3_13_tar",
    sha256 = "2c2e2a9867c1c61697ea0d8c0f74c7e9f1b1cf53b75dff95ca3bc03feb19ea7e",
    urls = ["https://github.com/coreos/etcd/releases/download/v3.3.13/etcd-v3.3.13-linux-amd64.tar.gz"],
)

http_file(
    name = "etcd_3_3_17_tar",
    sha256 = "8c1168a24d17a2d6772f8148ea35d4f3398c51f1e23db90c849d506adb387060",
    urls = ["https://github.com/coreos/etcd/releases/download/v3.3.17/etcd-v3.3.17-linux-amd64.tar.gz"],
)

http_file(
    name = "etcd_3_4_3_tar",
    sha256 = "6c642b723a86941b99753dff6c00b26d3b033209b15ee33325dc8e7f4cd68f07",
    urls = ["https://github.com/coreos/etcd/releases/download/v3.4.3/etcd-v3.4.3-linux-amd64.tar.gz"],
)

#=============================================================================
# Build etcd from source
# This picks up a number of critical bug fixes, for example:
#  * Caching of /etc/hosts https://github.com/golang/go/issues/13340
#  * General GC etc improvements
#  * Misc security fixes that are not backported

# Download via HTTP
go_repository(
    name = "etcd_v2_2_1_source",
    urls = ["https://github.com/etcd-io/etcd/archive/v2.2.1.tar.gz"],
    sha256 = "1c0ce63812ef951f79c0a544c91f9f1ba3c6b50cb3e8197de555732454864d05",
    importpath = "github.com/coreos/etcd",
    strip_prefix = "etcd-2.2.1/",
    build_external = "vendored",
    build_file_proto_mode = "disable_global",
)
