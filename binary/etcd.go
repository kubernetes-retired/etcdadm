package binary

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/platform9/etcdadm/util"
)

// EnsureInstalled installs etcd if it is not installed
func EnsureInstalled(releaseURL, version, installDir string) error {
	installed, err := isInstalled(version, installDir)
	if err != nil {
		return fmt.Errorf("unable to verify that etcd is installed: %s", err)
	}
	if installed {
		return nil
	}
	err = install(releaseURL, version, installDir)
	if err != nil {
		return err
	}
	installed, err = isInstalled(version, installDir)
	if err != nil {
		return fmt.Errorf("unable to verify that etcd is installed: %s", err)
	}
	return nil
}

func isInstalled(version, installDir string) (bool, error) {
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

func isEtcdInstalled(version, installDir string) (bool, error) {
	path := filepath.Join(installDir, "etcd")
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

func isEtcdctlInstalled(version, installDir string) (bool, error) {
	path := filepath.Join(installDir, "etcdctl")
	exists, err := util.FileExists(path)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	cmd := exec.Command(path, "version")
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

func extract(installDir, archive string) error {
	log.Printf("[install] extracting etcd archive %s to %s\n", archive, installDir)
	cmd := exec.Command("tar", "xzf", archive, "--strip-components=1", "-C", installDir)
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

func install(releaseURL, version, installDir string) error {
	log.Printf("[install] installing etcd %s from %s to %s\n", releaseURL, version, installDir)
	err := os.MkdirAll(installDir, 0700)
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

	err = extract(installDir, archive)
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
