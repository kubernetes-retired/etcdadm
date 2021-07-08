/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package etcdversions

import (
	"fmt"

	"github.com/blang/semver/v4"
	"k8s.io/klog/v2"
)

// By declaring the versions here, using constants, we likely force a compilation error
// on an inconsistent update

const (
	Version_3_2_24 = "3.2.24"
	Version_3_3_17 = "3.3.17"
	Version_3_4_3  = "3.4.3"
	Version_3_4_13 = "3.4.13"
	Version_3_5_0  = "3.5.0"
)

var AllEtcdVersions = []string{
	Version_3_2_24,
	Version_3_3_17,
	Version_3_4_3,
	Version_3_4_13,
	Version_3_5_0,
}

func UpgradeInPlaceSupported(fromVersion, toVersion string) bool {
	fromSemver, err := semver.ParseTolerant(fromVersion)
	if err != nil {
		klog.Warningf("unknown version format: %q", fromVersion)
		return false
	}

	toSemver, err := semver.ParseTolerant(toVersion)
	if err != nil {
		klog.Warningf("unknown version format: %q", toVersion)
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
		if fromSemver.Minor == 3 && toSemver.Minor == 4 {
			return true
		}
		if fromSemver.Minor == 4 && toSemver.Minor == 4 {
			return true
		}
	}

	return false
}

func EtcdVersionForAdoption(fromVersion string) string {
	fromSemver, err := semver.ParseTolerant(fromVersion)
	if err != nil {
		klog.Warningf("unknown version format: %q", fromVersion)
		return ""
	}

	family := fmt.Sprintf("%d.%d", fromSemver.Major, fromSemver.Minor)
	switch family {
	case "3.0", "3.1", "3.2":
		return Version_3_2_24
	case "3.3":
		return Version_3_3_17
	case "3.4":
		if fromSemver.Patch <= 3 {
			return Version_3_4_3
		} else {
			return Version_3_4_13
		}
	case "3.5":
		return Version_3_5_0
	default:
		return ""
	}
}

func EtcdVersionForRestore(fromVersion string) string {
	fromSemver, err := semver.ParseTolerant(fromVersion)
	if err != nil {
		klog.Warningf("unknown version format: %q", fromVersion)
		return ""
	}

	family := fmt.Sprintf("%d.%d", fromSemver.Major, fromSemver.Minor)
	switch family {
	case "3.0", "3.1", "3.2":
		return Version_3_2_24
	case "3.3":
		return Version_3_3_17
	case "3.4":
		if fromSemver.Patch <= 3 {
			return Version_3_4_3
		} else {
			return Version_3_4_13
		}
	case "3.5":
		return Version_3_5_0
	default:
		return ""
	}
}
