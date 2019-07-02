/*
Copyright 2018 The Kubernetes Authors.

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

package binary

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	log "sigs.k8s.io/etcdadm/pkg/logrus"

	"sigs.k8s.io/etcdadm/constants"
	"sigs.k8s.io/etcdadm/util"
)

// IsInstalled method check if required etcd binaries are installed
func IsInstalled(version, installDir string) (bool, error) {
	log.Printf("[install] verifying etcd %s is installed in %s\n", version, installDir)
	installed, err := isEtcdInstalled(version, installDir)
	if err != nil {
		return false, err
	}
	if !installed {
		return false, nil
	}
	return isEtcdctlInstalled(version, installDir)
}

func isEtcdInstalled(version, inputDir string) (bool, error) {
	path := filepath.Join(inputDir, "etcd")
	exists, err := util.Exists(path)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	cmd := exec.Command(path, "--version")
	return util.CmdOutputContains(cmd, fmt.Sprintf("etcd Version: %s", version))
}

func isEtcdctlInstalled(version, inputDir string) (bool, error) {
	path := filepath.Join(inputDir, "etcdctl")
	exists, err := util.Exists(path)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	cmdVersionFlag := "--version"
	if os.Getenv("ETCDCTL_API") == "3" {
		cmdVersionFlag = "version"
	}
	cmd := exec.Command(path, cmdVersionFlag)
	return util.CmdOutputContains(cmd, fmt.Sprintf("etcdctl version: %s", version))
}

func get(url, archive string) error {
	log.Printf("[install] downloading etcd from %s to %s\n", url, archive)
	argStr := fmt.Sprintf("--connect-timeout %v --progress-bar --location --output %v %v", int(constants.DefaultDownloadConnectTimeout/time.Second), archive, url)
	cmd := exec.Command("curl", strings.Fields(argStr)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return err
	}
	err = cmd.Wait()
	if err != nil {
		return err
	}
	return nil
}

func extract(extractDir, archive string) error {
	log.Printf("[install] extracting etcd archive %s to %s\n", archive, extractDir)
	cmd := exec.Command("tar", "xzf", archive, "--strip-components=1", "-C", extractDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return err
	}
	err = cmd.Wait()
	if err != nil {
		return err
	}
	return nil
}

// Download installs the etcd binaries in the directory specified by locationDir
func Download(releaseURL, version, locationDir string) error {
	log.Printf("[install] Downloading & installing etcd %s from %s to %s\n", releaseURL, version, locationDir)
	if err := os.MkdirAll(locationDir, 0755); err != nil {
		return fmt.Errorf("unable to create install directory: %s", err)
	}
	archive := filepath.Join(locationDir, releaseFile(version))
	url := downloadURL(releaseURL, version)
	if err := get(url, archive); err != nil {
		return fmt.Errorf("unable to download etcd: %s", err)
	}
	return nil
}

func releaseFile(version string) string {
	return fmt.Sprintf("etcd-v%s-linux-amd64.tar.gz", version)
}

func downloadURL(releaseURL, version string) string {
	// FIXME use url.ResolveReference to join
	return fmt.Sprintf("%s/v%s/%s", releaseURL, version, releaseFile(version))
}

// InstallFromCache method installs the binaries from cache directory
func InstallFromCache(version, installDir, cacheDir string) (bool, error) {
	archive := filepath.Join(cacheDir, releaseFile(version))
	if _, err := os.Stat(archive); os.IsNotExist(err) {
		return false, nil
	}
	// Create a tmp dir
	tmpDir, err := ioutil.TempDir("", "etcd")
	if err != nil {
		return true, fmt.Errorf("unable to create tmp dir %s to extract etcd archive", err)
	}
	defer os.RemoveAll(tmpDir)
	// Extract tar to tmp location
	if err := extract(tmpDir, archive); err != nil {
		return true, fmt.Errorf("unable to extract etcd archive: %s", err)
	}
	// Create the install dir
	os.MkdirAll(installDir, 755)
	// Copy binaries
	if err := Install(tmpDir, installDir); err != nil {
		return false, fmt.Errorf("unable to copy binaries: %s", err)
	}
	return true, nil
}

//Install copies binaries from srcDir to installDir
func Install(srcDir, installDir string) error {
	etcdSrcPath := filepath.Join(srcDir, "etcd")
	etcdDestPath := filepath.Join(installDir, "etcd")
	etcdctlSrcPath := filepath.Join(srcDir, "etcdctl")
	etcdctlDestPath := filepath.Join(installDir, "etcdctl")
	if err := util.CopyFile(etcdSrcPath, etcdDestPath); err != nil {
		return err
	}
	return util.CopyFile(etcdctlSrcPath, etcdctlDestPath)
}

// Uninstall removes installed binaries and symlinks
func Uninstall(version, installDir string) error {
	// Remove binaries
	etcdPath := filepath.Join(installDir, "etcd")
	etcdctlPath := filepath.Join(installDir, "etcdctl")
	if err := os.Remove(etcdPath); err != nil {
		return err
	}
	return os.Remove(etcdctlPath)
}
