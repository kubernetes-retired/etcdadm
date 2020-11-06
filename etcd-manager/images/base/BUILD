package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_image",
    "container_layer",
    "container_push",
)

packages = [
    "e2fsprogs", # For mkfs.ext4
    "libext2fs2", # needed by e2fsprogs
    "libcom-err2", # needed by e2fsprogs
    "libblkid1", # needed by e2fsprogs
    "libuuid1", # needed by e2fsprogs

    "util-linux", # For blkid

    "mount", # For mount & umount
    "libmount1", # Needed by mount
    "libselinux1", # Needed by mount
    "libpcre3", # Needed by libselinux1

    "dash", # Needed for shell (for dual logging)

    "coreutils", # General tools
    "libacl1", # Needed by coreutils
    "libattr1", # Needed by coreutils
]

architectures = [
    "amd64",
    "arm64",
] 

[genrule(
    name = "pkg_tar_%s_%s" % (pkg, arch),
    srcs = ["@debian_packages_gz_%s//file" % (arch)],
    outs = ["%s_%s.tar" % (pkg, arch)],
    tools = ["@deb_tools//:deb-tools"],
    cmd = "./$(location @deb_tools//:deb-tools) --cache-dir $$TMPDIR --mirror http://snapshot.debian.org/archive/debian/20201101T154040Z/ --out \"$@\" --package %s --packages ./$(location @debian_packages_gz_%s//file)" % (pkg, arch),
    visibility = ["//visibility:public"],
) for pkg in packages for arch in architectures]

[container_layer(
    name = "pkg_layer_%s_%s" % (pkg, arch),
    tars = [
        ":pkg_tar_%s_%s" % (pkg, arch),
    ],
) for pkg in packages for arch in architectures]

[container_image(
    name = "base_image_%s" % (arch),
    base = "@distroless-base-%s//image" % (arch),
    tars = [":pkg_tar_%s_%s" % (pkg, arch) for pkg in packages],
 ) for arch in architectures]
