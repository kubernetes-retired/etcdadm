package util

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/coreos/etcd/etcdserver/etcdserverpb"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/platform9/etcdadm/apis"
	"github.com/platform9/etcdadm/constants"
)

func getEtcdClientV3(endpoint string, etcdAdmConfig *apis.EtcdAdmConfig) (*clientv3.Client, error) {
	tlsInfo := transport.TLSInfo{
		CertFile:      filepath.Join(etcdAdmConfig.CertificatesDir, constants.EtcdctlClientCertName),
		KeyFile:       filepath.Join(etcdAdmConfig.CertificatesDir, constants.EtcdctlClientKeyName),
		TrustedCAFile: filepath.Join(etcdAdmConfig.CertificatesDir, constants.EtcdCACertName),
	}
	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		log.Fatal(err)
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{endpoint},
		DialTimeout: 5 * time.Second,
		TLS:         tlsConfig,
	})
	return cli, err
}

// AddSelfToEtcdCluster adds the local node as a member to the etcd cluster
func AddSelfToEtcdCluster(endpoint string, etcdAdmConfig *apis.EtcdAdmConfig) (*clientv3.MemberAddResponse, error) {
	cli, err := getEtcdClientV3(endpoint, etcdAdmConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close()

	mresp, err := cli.MemberAdd(context.Background(), strings.Split(etcdAdmConfig.AdvertisePeerURLs.String(), ","))
	if err != nil {
		log.Fatalf("[cluster] Error: failed to add member with peerURLs %q to cluster: %s", etcdAdmConfig.AdvertisePeerURLs, err)
	}
	log.Printf("[cluster] added member with ID %d, peerURLs %q to cluster", mresp.Member.ID, etcdAdmConfig.AdvertisePeerURLs)
	return mresp, err
}

// RemoveSelfFromEtcdCluster removes the local node (self) from the etcd cluster
func RemoveSelfFromEtcdCluster(etcdAdmConfig *apis.EtcdAdmConfig) error {
	etcdEndpoint := etcdAdmConfig.LoopbackClientURL.String()
	cli, err := getEtcdClientV3(etcdEndpoint, etcdAdmConfig)
	if err != nil {
		return fmt.Errorf("[cluster] Error: etcdclient failed to connect: %s", err)
	}
	defer cli.Close()

	memberListResp, err := MemberList(etcdEndpoint, etcdAdmConfig)
	if err != nil {
		return err
	}
	if len(memberListResp.Members) == 1 {
		// If this is the only member (single etcd cluster), continue with remove, i.e. this method is noop
		log.Printf("[cluster] This is the only etcd member in the cluster, continuing remove.")
		return nil
	}

	// Find the memberID of local member
	memberID := memberListResp.Header.GetMemberId()

	// Remove member from etcd cluster
	_, err = cli.MemberRemove(context.Background(), memberID)
	if err != nil {
		return fmt.Errorf("[cluster] Error: failed to remove self from etcd cluster: %s", err)
	}
	log.Printf("[cluster] Removed self (member) with ID %d, from cluster", memberID)
	return nil
}

// MemberList lists the members that are part of the etcd cluster
func MemberList(endpoint string, etcdAdmConfig *apis.EtcdAdmConfig) (*clientv3.MemberListResponse, error) {
	cli, err := getEtcdClientV3(endpoint, etcdAdmConfig)
	if err != nil {
		return nil, fmt.Errorf("[cluster] Error: etcdclient failed to connect: %s", err)
	}
	defer cli.Close()
	resp, err := cli.MemberList(context.Background())
	return resp, err
}

func SelfMember(etcdAdmConfig *apis.EtcdAdmConfig) (*etcdserverpb.Member, error) {
	endpoint := etcdAdmConfig.LoopbackClientURL.String()

	cli, err := getEtcdClientV3(endpoint, etcdAdmConfig)
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	mresp, err := cli.MemberList(context.Background())
	if err != nil {
		return nil, err
	}

	endpointMemberID := mresp.Header.GetMemberId()

	for _, memb := range mresp.Members {
		if memb.ID == endpointMemberID {
			return memb, nil
		}
	}
	return nil, fmt.Errorf("member with ID %q not found in list", endpointMemberID)
}
