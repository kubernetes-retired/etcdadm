package etcd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
)

type etcdProcessMember struct {
	Id         string   `json:"id,omitempty"`
	Name       string   `json:"name,omitempty"`
	PeerURLs   []string `json:"peerURLs,omitempty"`
	ClientURLs []string `json:"clientURLs,omitempty"`
}

func (m *etcdProcessMember) String() string {
	s, err := json.Marshal(m)
	if err != nil {
		return fmt.Sprintf("<error marshallling: %v>", err)
	}
	return string(s)
}

type etcdProcessMemberList struct {
	Members []*etcdProcessMember `json:"members"`
}

type etcdProcess struct {
	BinDir  string
	DataDir string

	CreateNewCluster bool
	Version          string

	//ClusterName string
	//NodeName string
	//AdvertiseHostname string
	//
	//PeerPort int
	//
	//ClientListenHost string
	//ClientPort int
	//
	//ClusterToken string

	Cluster *EtcdCluster

	cmd *exec.Cmd

	mutex     sync.Mutex
	exitError error
	exitState *os.ProcessState
}

func (p *etcdProcess) Stop() error {
	if p.cmd == nil {
		glog.Warningf("received Stop when process not running")
		return nil
	}
	if err := p.cmd.Process.Kill(); err != nil {
		p.mutex.Lock()
		if p.exitState != nil {
			return nil
		}
		p.mutex.Unlock()
		return fmt.Errorf("failed to kill process: ", err)
	}

	for {
		p.mutex.Lock()
		if p.exitState != nil {
			p.mutex.Unlock()
			return nil
		}
		p.mutex.Unlock()
		time.Sleep(100 * time.Millisecond)
	}

}
func (p *etcdProcess) Start() error {
	c := exec.Command(path.Join(p.BinDir, "etcd"))
	glog.Infof("executing command %s %s", c.Path, c.Args)

	//clientPort := p.Cluster.ClientPort
	//peerPort := p.Cluster.PeerPort
	//
	//clientListenHost := ""
	//if clientListenHost == "" {
	//	clientListenHost = "0.0.0.0"
	//}

	//advertiseHostname := p.Cluster.Me.InternalName
	//if advertiseHostname == "" {
	//	advertiseHostname = "0.0.0.0"
	//}

	env := make(map[string]string)
	env["ETCD_DATA_DIR"] = p.DataDir
	env["ETCD_LISTEN_PEER_URLS"] = strings.Join(p.Cluster.Me.PeerUrls, ",")
	env["ETCD_LISTEN_CLIENT_URLS"] = strings.Join(p.Cluster.Me.ClientUrls, ",")
	env["ETCD_ADVERTISE_CLIENT_URLS"] = strings.Join(p.Cluster.Me.ClientUrls, ",")
	env["ETCD_INITIAL_ADVERTISE_PEER_URLS"] = strings.Join(p.Cluster.Me.PeerUrls, ",")

	if p.CreateNewCluster {
		env["ETCD_INITIAL_CLUSTER_STATE"] = "new"
	} else {
		env["ETCD_INITIAL_CLUSTER_STATE"] = "existing"
	}

	env["ETCD_NAME"] = p.Cluster.Me.Name
	if p.Cluster.ClusterToken != "" {
		env["ETCD_INITIAL_CLUSTER_TOKEN"] = p.Cluster.ClusterToken
	}

	var initialCluster []string
	for _, node := range p.Cluster.Nodes {
		initialCluster = append(initialCluster, node.Name+"="+strings.Join(node.PeerUrls, ","))
	}
	env["ETCD_INITIAL_CLUSTER"] = strings.Join(initialCluster, ",")

	for k, v := range env {
		c.Env = append(c.Env, k+"="+v)
	}
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	err := c.Start()
	if err != nil {
		return fmt.Errorf("error starting etcd: %v", err)
	}
	go func() {
		processState, err := p.cmd.Process.Wait()
		if err != nil {
			glog.Warningf("etcd exited error: %v", err)
		}
		p.mutex.Lock()
		p.exitState = processState
		p.exitError = err
		p.mutex.Unlock()
	}()

	p.cmd = c
	return nil
}

func (e *etcdProcess) listMembers() ([]*etcdProcessMember, error) {
	client := &http.Client{}
	method := "GET"
	url := fmt.Sprintf("http://127.0.0.1:%d/v2/members", e.Cluster.ClientPort)
	request, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error building etcd request %s %s: %v", method, url, err)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error performing etcd request %s %s: %v", method, url, err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected response querying etcd members %s %s: %s", method, url, response.Status)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading etcd response %s %s: %v", method, url, err)
	}
	members := &etcdProcessMemberList{}
	if err := json.Unmarshal(body, &members); err != nil {
		glog.Infof("invalid etcd response: %q", string(body))
		return nil, fmt.Errorf("error parsing etcd response %s %s: %v", method, url, err)
	}
	return members.Members, nil
}

func (e *etcdProcess) addMember(name string, peerURLs []string) (*etcdProcessMember, error) {
	client := &http.Client{}

	m := &etcdProcessMember{
		Name:     name,
		PeerURLs: peerURLs,
	}
	postBody, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("error building payload for member-add: %v", err)
	}
	method := "POST"
	url := fmt.Sprintf("http://127.0.0.1:%d/v2/members", e.Cluster.ClientPort)
	request, err := http.NewRequest("POST", url, bytes.NewReader(postBody))
	if err != nil {
		return nil, fmt.Errorf("error building etcd request %s %s: %v", method, url, err)
	}
	request.Header.Add("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error performing etcd request %s %s: %v", method, url, err)
	}
	defer response.Body.Close()
	if response.StatusCode != 201 {
		glog.Infof("POSTed content was %q", string(postBody))
		return nil, fmt.Errorf("unexpected response adding etcd member %s %s: %s", method, url, response.Status)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading etcd response %s %s: %v", method, url, err)
	}
	member := &etcdProcessMember{}
	if err := json.Unmarshal(body, &member); err != nil {
		glog.Infof("invalid etcd response: %q", string(body))
		return nil, fmt.Errorf("error parsing etcd response %s %s: %v", method, url, err)
	}
	glog.Infof("created etcd member: %v", member)
	return member, nil
}
