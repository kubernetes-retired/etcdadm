package binary

import (
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"path"
)

func Install(version string) error {
	file := fmt.Sprintf("etcd-%s-linux-amd64.tar.gz", version)
	url := fmt.Sprintf("https://github.com/coreos/etcd/releases/download/%s/%s", version, file)

	tempDir, err := ioutil.TempDir("/tmp", "etcdadm")
	if err != nil {
		return fmt.Errorf("unable to create temporary directory: %s", err)
	}
	tempFile := path.Join(tempDir, file)

	cmd := exec.Command("curl", "-sSL", "-o", tempFile, url)
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("unable to download etcd: %s", err)
	}
	log.Printf("Downloading etcd %s from %s", version, url)
	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("unable to download etcd: %s", err)
	}

	cmd = exec.Command("tar", "-xzv", "--strip-components=1", "-C", "/usr/local/bin", tempFile)
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("unable to install etcd: %s", err)
	}
	log.Printf("Installing etcd to /usr/local/bin")
	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("unable to install etcd: %s", err)
	}
	return nil
}
