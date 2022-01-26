load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

#=============================================================================
# Go rules

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "2b1641428dff9018f9e85c0384f03ec6c10660d935b750e3fa1492a281a53b0f",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.29.0/rules_go-v0.29.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.29.0/rules_go-v0.29.0.zip",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "62ca106be173579c0a167deb23358fdfe71ffa1e4cfdddf5582af26520f1c66f",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.23.0/bazel-gazelle-v0.23.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.23.0/bazel-gazelle-v0.23.0.tar.gz",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(
    go_version = "1.17.3",
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

gazelle_dependencies()

#=============================================================================
# Docker rules

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "1f4e59843b61981a96835dc4ac377ad4da9f8c334ebe5e0bb3f58f80c09735f4",
    strip_prefix = "rules_docker-0.19.0",
    urls = ["https://github.com/bazelbuild/rules_docker/releases/download/v0.19.0/rules_docker-v0.19.0.tar.gz"],
)

load(
    "@io_bazel_rules_docker//repositories:repositories.bzl",
    container_repositories = "repositories",
)

container_repositories()

load("@io_bazel_rules_docker//repositories:deps.bzl", container_deps = "deps")

container_deps()

load(
    "@io_bazel_rules_docker//repositories:go_repositories.bzl",
    docker_go_deps = "go_deps",
)

docker_go_deps()

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

http_file(
    name = "etcd_3.5.0_amd64_tar",
    sha256 = "864baa0437f8368e0713d44b83afe21dce1fb4ee7dae4ca0f9dd5f0df22d01c4",
    urls = ["https://github.com/etcd-io/etcd/releases/download/v3.5.0/etcd-v3.5.0-linux-amd64.tar.gz"],
)

http_file(
    name = "etcd_3.5.0_arm64_tar",
    sha256 = "444e10e6880595d75aaf55762901c722049b29d56fef50b2f23464bb7f9db74d",
    urls = ["https://github.com/etcd-io/etcd/releases/download/v3.5.0/etcd-v3.5.0-linux-arm64.tar.gz"],
)

http_file(
    name = "etcd_3.5.1_amd64_tar",
    sha256 = "728a14914217ce60de2e1299fc1a2c2c5564e7ffd0d9dadf3f5073103ab619b4",
    urls = ["https://github.com/etcd-io/etcd/releases/download/v3.5.1/etcd-v3.5.1-linux-amd64.tar.gz"],
)

http_file(
    name = "etcd_3.5.1_arm64_tar",
    sha256 = "86203022e23d7368bac23d96095270dc6300f356ea882e435926a9effd7e5f0e",
    urls = ["https://github.com/etcd-io/etcd/releases/download/v3.5.1/etcd-v3.5.1-linux-arm64.tar.gz"],
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
    digest = "sha256:7d57eac73dd3bbe097632d6b3b2cb1fee8368f8731ade38f49c67df9285bc473",
    registry = "gcr.io/distroless",
    repository = "base-debian10",
)

container_pull(
    name = "distroless-base-amd64-debug",
    architecture = "amd64",
    digest = "sha256:faeef0829d9736cf18ea3113898634ced46542c9d4445cbe15af8a19dda9ce0c",
    registry = "gcr.io/distroless",
    repository = "base-debian10",
)

container_pull(
    name = "distroless-base-arm64",
    architecture = "arm64",
    digest = "sha256:2b90e75fde5abf0ee5e5f5f9f34760005258bfb05e72cade687e46d6643b6c8b",
    registry = "gcr.io/distroless",
    repository = "base-debian10",
)

container_pull(
    name = "distroless-base-arm64-debug",
    architecture = "arm64",
    digest = "sha256:bb3ad4bd962ba6368c6c307939d9b94e4b79719c7b55e5a4a34b8d28360b5c23",
    registry = "gcr.io/distroless",
    repository = "base-debian10",
)

local_repository(
    name = "deb_tools",
    path = "tools/deb-tools",
)
