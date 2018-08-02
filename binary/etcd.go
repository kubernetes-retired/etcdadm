package binary

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/platform9/etcdadm/constants"
	"github.com/platform9/etcdadm/util"
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
	exists, err := util.FileExists(path)
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
	exists, err := util.FileExists(path)
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
	argStr := fmt.Sprintf("--connect-timeout %v --progress-bar --location --output %v %v", int(constants.DefaultDownloadConnectionTimeout/time.Second), archive, url)
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
	if err := os.MkdirAll(locationDir, 0700); err != nil {
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
	// Remove installDir if already present
	if err := os.RemoveAll(installDir); err != nil {
		return true, fmt.Errorf("unable to clean install directory: %s", err)
	}
	// Create installDir
	if err := os.MkdirAll(installDir, 0700); err != nil {
		return true, fmt.Errorf("unable to create install directory: %s", err)
	}
	// Extract tar to installDir
	if err := extract(installDir, archive); err != nil {
		return true, fmt.Errorf("unable to extract etcd archive: %s", err)
	}
	// Create symlinks
	if err := createSymLinks(installDir, constants.DefaultInstallBaseDir); err != nil {
		return false, fmt.Errorf("unable to create symlinks: %s", err)
	}
	return true, nil
}

func createSymLinks(installDir, symLinkDir string) error {
	etcdBinaryPath := filepath.Join(installDir, "etcd")
	etcdSymLinkPath := filepath.Join(symLinkDir, "etcd")
	etcdctlBinaryPath := filepath.Join(installDir, "etcdctl")
	etcdctlSymLinkPath := filepath.Join(symLinkDir, "etcdctl")

	if err := os.Symlink(etcdBinaryPath, etcdSymLinkPath); err != nil {
		return err
	}
	return os.Symlink(etcdctlBinaryPath, etcdctlSymLinkPath)
}

// DeleteSymLinks deletes symlinks created for etcd binaires
func DeleteSymLinks(symLinkDir string) error {
	etcdSymLinkPath := filepath.Join(symLinkDir, "etcd")
	etcdctlSymLinkPath := filepath.Join(symLinkDir, "etcdctl")
	if err := os.Remove(etcdSymLinkPath); err != nil {
		return err
	}
	return os.Remove(etcdctlSymLinkPath)
}
