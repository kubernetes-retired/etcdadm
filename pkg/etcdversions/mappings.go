package etcdversions

import (
	"fmt"

	"github.com/blang/semver"
	"github.com/golang/glog"
)

// By declaring the versions here, using constants, we likely force a compilation error
// on an inconsistent update

const (
	Version_2_2_1  = "2.2.1"
	Version_3_1_12 = "3.1.12"
	Version_3_2_18 = "3.2.18"
	Version_3_2_24 = "3.2.24"
	Version_3_3_10 = "3.3.10"
	Version_3_3_13 = "3.3.13"
)

var AllEtcdVersions = []string{
	Version_2_2_1,
	Version_3_1_12,
	Version_3_2_18,
	Version_3_2_24,
	Version_3_3_10,
	Version_3_3_13,
}

func UpgradeInPlaceSupported(fromVersion, toVersion string) bool {
	fromSemver, err := semver.ParseTolerant(fromVersion)
	if err != nil {
		glog.Warningf("unknown version format: %q", fromVersion)
		return false
	}

	toSemver, err := semver.ParseTolerant(toVersion)
	if err != nil {
		glog.Warningf("unknown version format: %q", toVersion)
		return false
	}

	if fromSemver.Major == 3 && toSemver.Major == 3 {
		if fromSemver.Minor == 0 && toSemver.Minor == 0 {
			return true
		}
		if fromSemver.Minor == 0 && toSemver.Minor == 1 {
			return true
		}
		if fromSemver.Minor == 1 && toSemver.Minor == 1 {
			return true
		}
		if fromSemver.Minor == 1 && toSemver.Minor == 2 {
			return true
		}
		if fromSemver.Minor == 2 && toSemver.Minor == 2 {
			return true
		}
		if fromSemver.Minor == 2 && toSemver.Minor == 3 {
			return true
		}
		if fromSemver.Minor == 3 && toSemver.Minor == 3 {
			return true
		}
	}

	return false
}

func EtcdVersionForAdoption(fromVersion string) string {
	fromSemver, err := semver.ParseTolerant(fromVersion)
	if err != nil {
		glog.Warningf("unknown version format: %q", fromVersion)
		return ""
	}

	family := fmt.Sprintf("%d.%d", fromSemver.Major, fromSemver.Minor)
	switch family {
	case "2.2":
		return Version_2_2_1
	case "3.0":
		return Version_3_1_12
	case "3.1":
		return Version_3_1_12
	case "3.2":
		if fromSemver.Patch <= 18 {
			return Version_3_2_18
		} else {
			return Version_3_2_24
		}
	case "3.3":
		if fromSemver.Patch <= 10 {
			return Version_3_3_10
		} else {
			return Version_3_3_13
		}
	default:
		return ""
	}
}

func EtcdVersionForRestore(fromVersion string) string {
	fromSemver, err := semver.ParseTolerant(fromVersion)
	if err != nil {
		glog.Warningf("unknown version format: %q", fromVersion)
		return ""
	}

	family := fmt.Sprintf("%d.%d", fromSemver.Major, fromSemver.Minor)
	switch family {
	case "2.2":
		return Version_2_2_1
	case "3.0":
		return Version_3_1_12
	case "3.1":
		return Version_3_1_12
	case "3.2":
		if fromSemver.Patch <= 18 {
			return Version_3_2_18
		} else {
			return Version_3_2_24
		}
	case "3.3":
		if fromSemver.Patch <= 10 {
			return Version_3_3_10
		} else {
			return Version_3_3_13
		}
	default:
		return ""
	}
}
