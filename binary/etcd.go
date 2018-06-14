package binary

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/platform9/etcdadm/constants"
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
	for _, binary := range []string{"etcd", "etcdctl"} {
		path := filepath.Join(installDir, binary)
		exists, err := util.FileExists(path)
		if err != nil {
			return false, err
		}
		if !exists {
			return false, nil
		}
		cmd := exec.Command(path, "--version")
		out, err := cmd.Output()
		if err != nil {
			return false, err
		}
		installed := strings.Contains(strings.ToLower(string(out)), fmt.Sprintf("%s version: %s", binary, version))
		if !installed {
			return false, nil
		}
	}
	return true, nil
}

func get(url, archive string) error {
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

	archive := filepath.Join(downloadDir, constants.ReleaseFile(version))

	url := constants.DownloadURL(releaseURL, version)
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
