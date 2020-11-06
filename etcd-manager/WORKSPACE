load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "d9d71a5fdfcf5f5326f1ffc4bcaea6519cb4fcfe0aaee6ae68c7440ee8b46bc8",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.22.7/rules_go-v0.22.7.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.22.7/rules_go-v0.22.7.tar.gz",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "cdb02a887a7187ea4d5a27452311a75ed8637379a1287d8eeb952138ea485f7d",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.21.1/bazel-gazelle-v0.21.1.tar.gz",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_toolchains")

go_rules_dependencies()

go_register_toolchains(
    go_version = "1.14.5",
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")

gazelle_dependencies()

#=============================================================================

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "6287241e033d247e9da5ff705dd6ef526bac39ae82f3d17de1b69f8cb313f9cd",
    strip_prefix = "rules_docker-0.14.3",
    urls = ["https://github.com/bazelbuild/rules_docker/releases/download/v0.14.3/rules_docker-v0.14.3.tar.gz"],
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

container_pull(
    name = "debian-hyperkube-base-arm64",
    architecture = "arm64",
    digest = "sha256:78eeb1a31eef7c16f954444d64636d939d89307e752964ad6d9d06966c722da3",
    registry = "k8s.gcr.io",
    repository = "debian-hyperkube-base",
    tag = "0.12.1",  # ignored, but kept here for documentation
)

#=============================================================================
# etcd rules

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_file")

# We build etcd 2.2 from source, because the released version has some critical problems,
# particularly visible around caching of /etc/hosts
#http_file(
#    name = "etcd_2.2.1_tar",
#    sha256 = "59f7985c81b6bc551246c165c2fd83e33a063875e4e0c61920b1d90a4910f462",
#    urls = ["https://github.com/coreos/etcd/releases/download/v2.2.1/etcd-v2.2.1-linux-amd64.tar.gz"],
#)

http_file(
    name = "etcd_3.1.12_amd64_tar",
    sha256 = "4b22184bef1bba8b4908b14bae6af4a6d33ec2b91e4f7a240780e07fa43f2111",
    urls = ["https://github.com/coreos/etcd/releases/download/v3.1.12/etcd-v3.1.12-linux-amd64.tar.gz"],
)

# 3.1.12 was not released for arm64
#http_file(
#    name = "etcd_3.1.12_arm64_tar",
#    sha256 = "4b22184bef1bba8b4908b14bae6af4a6d33ec2b91e4f7a240780e07fa43f2123",
#    urls = ["https://github.com/coreos/etcd/releases/download/v3.1.12/etcd-v3.1.12-linux-arm64.tar.gz"],
#)

http_file(
    name = "etcd_3.2.18_amd64_tar",
    sha256 = "b729db0732448064271ea6fdcb901773c4fe917763ca07776f22d0e5e0bd4097",
    urls = ["https://github.com/coreos/etcd/releases/download/v3.2.18/etcd-v3.2.18-linux-amd64.tar.gz"],
)

http_file(
    name = "etcd_3.2.18_arm64_tar",
    sha256 = "085c13764af02ca2762cbacade374583a532d4f75a7b996a62f67f8f044641e6",
    urls = ["https://github.com/coreos/etcd/releases/download/v3.2.18/etcd-v3.2.18-linux-arm64.tar.gz"],
)

http_file(
    name = "etcd_3.2.24_amd64_tar",
    sha256 = "947849dbcfa13927c81236fb76a7c01d587bbab42ab1e807184cd91b026ebed7",
    urls = ["https://github.com/coreos/etcd/releases/download/v3.2.24/etcd-v3.2.24-linux-amd64.tar.gz"],
)

http_file(
    name = "etcd_3.2.24_arm64_tar",
    sha256 = "7d3db622fb8d22a669a9351e1002ed2a7a776004a4a35888734bf39323889390",
    urls = ["https://github.com/coreos/etcd/releases/download/v3.2.24/etcd-v3.2.24-linux-arm64.tar.gz"],
)

http_file(
    name = "etcd_3.3.10_amd64_tar",
    sha256 = "1620a59150ec0a0124a65540e23891243feb2d9a628092fb1edcc23974724a45",
    urls = ["https://github.com/coreos/etcd/releases/download/v3.3.10/etcd-v3.3.10-linux-amd64.tar.gz"],
)

http_file(
    name = "etcd_3.3.10_arm64_tar",
    sha256 = "5ec97b0b872adce275b8130d19db314f7f2b803aeb24c4aae17a19e2d66853c4",
    urls = ["https://github.com/coreos/etcd/releases/download/v3.3.10/etcd-v3.3.10-linux-arm64.tar.gz"],
)

http_file(
    name = "etcd_3.3.13_amd64_tar",
    sha256 = "2c2e2a9867c1c61697ea0d8c0f74c7e9f1b1cf53b75dff95ca3bc03feb19ea7e",
    urls = ["https://github.com/coreos/etcd/releases/download/v3.3.13/etcd-v3.3.13-linux-amd64.tar.gz"],
)

http_file(
    name = "etcd_3.3.13_arm64_tar",
    sha256 = "ff76e534db8378f112b48c445944069fc9923bef04dae4d66e36801da13cc8a1",
    urls = ["https://github.com/coreos/etcd/releases/download/v3.3.13/etcd-v3.3.13-linux-arm64.tar.gz"],
)

http_file(
    name = "etcd_3.3.17_amd64_tar",
    sha256 = "8c1168a24d17a2d6772f8148ea35d4f3398c51f1e23db90c849d506adb387060",
    urls = ["https://github.com/coreos/etcd/releases/download/v3.3.17/etcd-v3.3.17-linux-amd64.tar.gz"],
)

http_file(
    name = "etcd_3.3.17_arm64_tar",
    sha256 = "0ea20dfbf3085f584f788287fd398979d0f1271549be6497d81ec635b9b4c121",
    urls = ["https://github.com/coreos/etcd/releases/download/v3.3.17/etcd-v3.3.17-linux-arm64.tar.gz"],
)

http_file(
    name = "etcd_3.4.3_amd64_tar",
    sha256 = "6c642b723a86941b99753dff6c00b26d3b033209b15ee33325dc8e7f4cd68f07",
    urls = ["https://github.com/coreos/etcd/releases/download/v3.4.3/etcd-v3.4.3-linux-amd64.tar.gz"],
)

http_file(
    name = "etcd_3.4.3_arm64_tar",
    sha256 = "01bd849ad99693600bd59db8d0e66ac64aac1e3801900665c31bd393972e3554",
    urls = ["https://github.com/coreos/etcd/releases/download/v3.4.3/etcd-v3.4.3-linux-arm64.tar.gz"],
)

http_file(
    name = "etcd_3.4.13_amd64_tar",
    sha256 = "2ac029e47bab752dacdb7b30032f230f49e2f457cbc32e8f555c2210bb5ff107",
    urls = ["https://github.com/etcd-io/etcd/releases/download/v3.4.13/etcd-v3.4.13-linux-amd64.tar.gz"],
)

http_file(
    name = "etcd_3.4.13_arm64_tar",
    sha256 = "1934ebb9f9f6501f706111b78e5e321a7ff8d7792d3d96a76e2d01874e42a300",
    urls = ["https://github.com/etcd-io/etcd/releases/download/v3.4.13/etcd-v3.4.13-linux-arm64.tar.gz"],
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

#=============================================================================

http_file(
    name = "debian_packages_gz_amd64",
    sha256 = "369d45f6c138af98d8ea8a598564dcabc1f6991ac777fb2d351e846f195cdc13",
    urls = ["http://snapshot.debian.org/archive/debian/20201101T154040Z/dists/buster/main/binary-amd64/Packages.gz"],
)

http_file(
    name = "debian_packages_gz_arm64",
    sha256 = "62a7e0c34f45a2524024ef4871e48f061f8d57d54e6f9d75d2aa2bff55ca91b8",
    urls = ["http://snapshot.debian.org/archive/debian/20201101T154040Z/dists/buster/main/binary-arm64/Packages.gz"],
)

container_pull(
    name = "distroless-base-amd64",
    architecture = "amd64",
    digest = "sha256:abe4b6cd34fed3ade2e89ed1f2ce75ddab023ea0d583206cfa4f960b74572c67",
    registry = "gcr.io/distroless",
    repository = "base-debian10",
)

container_pull(
    name = "distroless-base-amd64-debug",
    architecture = "amd64",
    digest = "sha256:ef82640400a9f1623813bd32cf23de4db317086bddcda1f4b1787a4ab57ec319",
    registry = "gcr.io/distroless",
    repository = "base-debian10",
)

container_pull(
    name = "distroless-base-arm64",
    architecture = "arm64",
    digest = "sha256:abe4b6cd34fed3ade2e89ed1f2ce75ddab023ea0d583206cfa4f960b74572c67",
    registry = "gcr.io/distroless",
    repository = "base-debian10",
)

container_pull(
    name = "distroless-base-arm64-debug",
    architecture = "arm64",
    digest = "sha256:ef82640400a9f1623813bd32cf23de4db317086bddcda1f4b1787a4ab57ec319",
    registry = "gcr.io/distroless",
    repository = "base-debian10",
)

local_repository(
    name = "deb_tools",
    path = "tools/deb-tools",
)
