package binary

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/platform9/etcdadm/constants"
	"github.com/platform9/etcdadm/util"
)

// EnsureInstalled installs etcd if it is not installed
func EnsureInstalled(releaseURL, version, installDir, cacheDir string) error {
	installed, err := isInstalled(version, installDir)
	if err != nil {
		return fmt.Errorf("unable to verify that etcd is installed: %s", err)
	}
	if installed {
		return nil
	}

	// If not installed, try to install it from cache first
	if err = installFromCache(releaseURL, version, installDir, cacheDir); err != nil {
		log.Printf("[install] install from cache errored! error=%s", err)
		// Install from cache failed. Try to download to cache first
		if err = Download(releaseURL, version, cacheDir); err != nil {
			return err
		}
		// Install from cache after download
		if err = installFromCache(releaseURL, version, installDir, cacheDir); err != nil {
			return err
		}
	}

	installed, err = isInstalled(version, installDir)
	if err != nil {
		return fmt.Errorf("unable to verify that etcd is installed: %s", err)
	}
	if !installed {
		return fmt.Errorf("etcd binaries not installed. Unable to download from upstream either")
	}

	if err = createSymLinks(installDir, constants.DefaultInstallBaseDir); err != nil {
		return fmt.Errorf("unable to create symlinks: %s", err)
	}
	return nil
}

func isInstalled(version, inputDir string) (bool, error) {
	log.Printf("[install] verifying etcd %s is installed in %s\n", version, inputDir)
	installed, err := isEtcdInstalled(version, inputDir)
	if err != nil {
		return false, err
	}
	if !installed {
		return false, nil
	}
	return isEtcdctlInstalled(version, inputDir)
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
	cmd := exec.Command("curl", "-Lo", archive, url)
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
	err := os.MkdirAll(locationDir, 0700)
	if err != nil {
		return fmt.Errorf("unable to create install directory: %s", err)
	}

	if err != nil {
		return fmt.Errorf("unable to create install directory: %s", err)
	}

	downloadDir, err := ioutil.TempDir("/tmp", "etcdadm")
	if err != nil {
		return fmt.Errorf("unable to create temporary directory: %s", err)
	}
	defer os.RemoveAll(downloadDir)

	archive := filepath.Join(downloadDir, releaseFile(version))

	url := downloadURL(releaseURL, version)
	err = get(url, archive)
	if err != nil {
		return fmt.Errorf("unable to download etcd: %s", err)
	}

	err = extract(locationDir, archive)
	if err != nil {
		return fmt.Errorf("unable to extract etcd archive: %s", err)
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

func installFromCache(releaseURL, version, installDir, cacheDir string) error {
	// Create installDir if not already present
	err := os.MkdirAll(installDir, 0700)
	if err != nil {
		return fmt.Errorf("unable to create install directory: %s", err)
	}
	installed, err := isInstalled(version, cacheDir)
	if !installed {
		return fmt.Errorf("etcd %s version binaries not found in cache dir %s", version, cacheDir)
	}
	// move contents from cacheDir to installDir
	if err := util.CopyRecursive(fmt.Sprintf("%s/.", cacheDir), installDir); err != nil {
		return err
	}
	return nil
}

func createSymLinks(installDir, symLinkDir string) error {
	etcdBinaryPath := filepath.Join(installDir, "etcd")
	etcdSymLinkPath := filepath.Join(symLinkDir, "etcd")
	etcdCtlBinaryPath := filepath.Join(installDir, "etcdctl")
	etcdctlSymLinkPath := filepath.Join(symLinkDir, "etcdctl")

	if err := util.CreateSymLink(etcdBinaryPath, etcdSymLinkPath); err != nil {
		return err
	}
	return util.CreateSymLink(etcdCtlBinaryPath, etcdctlSymLinkPath)
}

// DeleteSymLinks deletes symlinks created for etcd binaires
func DeleteSymLinks(symLinkDir string) error {
	etcdSymLinkPath := filepath.Join(symLinkDir, "etcd")
	etcdctlSymLinkPath := filepath.Join(symLinkDir, "etcdctl")
	if err := util.RemoveFile(etcdSymLinkPath); err != nil {
		return err
	}
	return util.RemoveFile(etcdctlSymLinkPath)
}
