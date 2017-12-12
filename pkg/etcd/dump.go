package etcd

import (
	"archive/tar"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	etcd_client "github.com/coreos/etcd/client"
	"github.com/golang/glog"
)

func DumpBackup(dataDir string, tw *tar.Writer) error {
	clientURL := "http://127.0.0.1:4001"
	peerURL := "http://127.0.0.1:2379"

	binDir := "/opt/etcd-v2.2.1-linux-amd64"

	c := exec.Command(path.Join(binDir, "etcd"))
	c.Args = append(c.Args, "--force-new-cluster")
	c.Args = append(c.Args, "--data-dir", dataDir)
	c.Args = append(c.Args, "--listen-client-urls", clientURL)
	c.Args = append(c.Args, "--advertise-client-urls", clientURL)
	c.Args = append(c.Args, "--listen-peer-urls", peerURL)

	env := make(map[string]string)
	for k, v := range env {
		c.Env = append(c.Env, k+"="+v)
	}

	glog.Infof("executing command %s %s", c.Path, c.Args)

	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Start(); err != nil {
		return fmt.Errorf("error starting etcd backup: %v", err)
	}

	stopped := false
	defer func() {
		if !stopped {
			if err := c.Process.Kill(); err != nil {
				glog.Warningf("error stopping etcd: %v", err)
			}
		}
	}()

	cfg := etcd_client.Config{
		Endpoints:               []string{clientURL},
		Transport:               etcd_client.DefaultTransport,
		HeaderTimeoutPerRequest: 10 * time.Second,
	}
	etcdClient, err := etcd_client.New(cfg)
	if err != nil {
		return fmt.Errorf("error building etcd client for %s: %v", clientURL, err)
	}

	keysAPI := etcd_client.NewKeysAPI(etcdClient)

	for i := 0; i < 60; i++ {
		ctx := context.TODO()
		_, err := keysAPI.Get(ctx, "/", &etcd_client.GetOptions{Quorum: false})
		if err == nil {
			break
		}
		glog.Infof("Waiting for etcd to start (%v)", err)
		time.Sleep(time.Second)
	}

	if err := dumpRecursive(keysAPI, "/", tw); err != nil {
		return fmt.Errorf("error dumping keys: %v", err)
	}

	if err := c.Process.Kill(); err != nil {
		return fmt.Errorf("error stopping etcd: %v", err)
	}
	stopped = true
	return nil
}

func dumpRecursive(keys etcd_client.KeysAPI, p string, tw *tar.Writer) error {
	ctx := context.TODO()
	opts := &etcd_client.GetOptions{
		Quorum: false,
		// We don't do Recursive: true, to avoid huge responses
	}
	response, err := keys.Get(ctx, p, opts)
	if err != nil {
		return fmt.Errorf("error reading %q: %v", p, err)
	}

	if response.Node == nil {
		return fmt.Errorf("node %q not found", p)
	}

	hdr := &tar.Header{
		Name: response.Node.Key,
	}
	if response.Node.Dir {
		hdr.Mode = 0755
		hdr.Name += "/"
		hdr.Typeflag = tar.TypeDir
	} else {
		hdr.Mode = 0644
		hdr.Typeflag = tar.TypeReg
		hdr.Size = int64(len(response.Node.Value))
	}

	if hdr.Name != "/" {
		hdr.Name = strings.TrimPrefix(hdr.Name, "/")

		// write the header
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("error writing tar header: %v", err)
		}

		if !response.Node.Dir && response.Node.Value != "" {
			_, err := tw.Write([]byte(response.Node.Value))
			if err != nil {
				return fmt.Errorf("error writing tar data: %v", err)
			}
		}
	}

	for _, n := range response.Node.Nodes {
		err := dumpRecursive(keys, n.Key, tw)
		if err != nil {
			return err
		}
	}

	return nil
}
