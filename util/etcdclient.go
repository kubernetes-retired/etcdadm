package util

import (
	"context"
	"fmt"
	"log"
	"net/url"
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
	etcdEndpoint := fmt.Sprintf("https://%s:%d", constants.DefaultLoopbackHost, constants.DefaultClientPort)
	cli, err := getEtcdClientV3(etcdEndpoint, etcdAdmConfig)
	if err != nil {
		log.Print(err)
		return err
	}
	defer cli.Close()
	apis.DefaultAdvertisePeerURLs(etcdAdmConfig)
	members, err := MemberList(etcdEndpoint, etcdAdmConfig)
	// Find the current member from the list and extract it's ID
	for _, m := range members.Members {
		for _, urlString := range m.PeerURLs {
			peerurl, _ := url.Parse(urlString)
			if *peerurl == etcdAdmConfig.AdvertisePeerURLs[0] {
				memberID := m.GetID()
				_, err := cli.MemberRemove(context.Background(), memberID)
				if err != nil {
					log.Fatalf("[cluster] Error: failed to remove self from etcd cluster: %s", err)
				}
				log.Printf("[cluster] Removed self (member) with ID %d, from cluster", memberID)
				return err
			}
		}
	}
	log.Fatalf("[cluster] Error: failed to remove self from etcd cluster. Self not in cluster members: %s", err)
	return err
}

// MemberList lists the members that are part of the etcd cluster
func MemberList(endpoint string, etcdAdmConfig *apis.EtcdAdmConfig) (*clientv3.MemberListResponse, error) {
	cli, err := getEtcdClientV3(endpoint, etcdAdmConfig)
	if err != nil {
		log.Fatal(err)
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

	sresp, err := cli.Status(context.Background(), endpoint)
	if err != nil {
		return nil, err
	}
	endpointMemberID := sresp.Header.MemberId

	mresp, err := cli.MemberList(context.Background())
	if err != nil {
		return nil, err
	}

	for _, memb := range mresp.Members {
		if memb.ID == endpointMemberID {
			return memb, nil
		}
	}
	return nil, fmt.Errorf("member with ID %q not found in list", endpointMemberID)
}
