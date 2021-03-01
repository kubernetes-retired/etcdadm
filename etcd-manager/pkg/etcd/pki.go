/*
Copyright 2020 The Kubernetes Authors.

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

package etcd

import (
	"crypto/x509"
	"fmt"
	"net"
	"net/url"
	"path/filepath"

	certutil "k8s.io/client-go/util/cert"
	"k8s.io/klog/v2"
	protoetcd "sigs.k8s.io/etcdadm/etcd-manager/pkg/apis/etcd"
	"sigs.k8s.io/etcdadm/etcd-manager/pkg/pki"
)

func (p *etcdProcess) createKeypairs(peersCA *pki.Keypair, clientsCA *pki.Keypair, pkiDir string, me *protoetcd.EtcdNode, peerClientIPs []net.IP) error {
	if peersCA != nil {
		peersDir := filepath.Join(pkiDir, "peers")
		p.PKIPeersDir = peersDir

		// Create a peer certificate
		store := pki.NewFSStore(peersDir)
		if err := store.WriteCertificate("ca", peersCA); err != nil {
			return err
		}

		keypairs := pki.Keypairs{Store: store}
		keypairs.SetCA(peersCA)

		certConfig := certutil.Config{
			CommonName: me.Name,
			AltNames: certutil.AltNames{
				DNSNames: []string{me.Name},
			},
			Usages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		}

		// etcd 3.2 does some deeper client-certiifcate validation, so we want to include our client IP address
		// (as the DNS name might not be immediately ready, see https://github.com/kopeio/etcd-manager/issues/371)
		for _, ip := range peerClientIPs {
			certConfig.AltNames.IPs = append(certConfig.AltNames.IPs, ip)
		}
		klog.Infof("adding peerClientIPs %v", peerClientIPs)

		if err := addAltNames(&certConfig, me.PeerUrls); err != nil {
			return err
		}

		certConfig.AltNames.IPs = removeDuplicateIPs(certConfig.AltNames.IPs)

		klog.Infof("generating peer keypair for etcd: %+v", certConfig)

		_, err := keypairs.EnsureKeypair("me", certConfig, peersCA)
		if err != nil {
			return err
		}
	} else {
		klog.Warningf("not generating peer keypair as peers-ca not set")
	}

	p.etcdClientsCA = clientsCA

	if clientsCA != nil {
		clientsDir := filepath.Join(pkiDir, "clients")
		p.PKIClientsDir = clientsDir

		// Create a client certificate
		store := pki.NewFSStore(clientsDir)
		if err := store.WriteCertificate("ca", clientsCA); err != nil {
			return err
		}

		keypairs := pki.Keypairs{Store: store}
		keypairs.SetCA(clientsCA)

		// The server cert is used by the gRPC library of etcd as a client cert for meta checks, like health
		// See https://github.com/etcd-io/etcd/issues/9785
		certConfig := certutil.Config{
			CommonName: me.Name,
			AltNames: certutil.AltNames{
				DNSNames: []string{me.Name},
			},
			Usages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		}

		if err := addAltNames(&certConfig, me.ClientUrls); err != nil {
			return err
		}

		if err := addAltNames(&certConfig, me.PeerUrls); err != nil {
			return err
		}

		klog.Infof("building client-serving certificate: %+v", certConfig)

		_, err := keypairs.EnsureKeypair("server", certConfig, clientsCA)
		if err != nil {
			return err
		}
	} else {
		klog.Warningf("not generating client keypair as clients-ca not set")
	}

	if clientsCA != nil {
		store := pki.NewInMemoryStore()
		keypairs := &pki.Keypairs{Store: store}
		keypairs.SetCA(clientsCA)

		c, err := BuildTLSClientConfig(keypairs, me.Name)
		if err != nil {
			return err
		}
		p.etcdClientTLSConfig = c
	}

	return nil
}

func addAltNames(certConfig *certutil.Config, urls []string) error {
	for _, urlString := range urls {
		u, err := url.Parse(urlString)
		if err != nil {
			return fmt.Errorf("cannot parse url: %q", urlString)
		}

		switch u.Scheme {
		case "http", "https":
			// We generate even for http endpoints, because that way we can switch to https dynamically
			h := u.Hostname() // Hostname does not include port
			ip := net.ParseIP(h)
			if ip == nil {
				certConfig.AltNames.DNSNames = append(certConfig.AltNames.DNSNames, h)
			} else {
				certConfig.AltNames.IPs = append(certConfig.AltNames.IPs, ip)
			}

		default:
			return fmt.Errorf("unknown URL %q", urlString)
		}
	}

	// We always self-sign for 127.0.0.1, so that we can always be reached by apiserver / debug clients
	// sometimes it will be there already
	for _, ip := range certConfig.AltNames.IPs {
		if ip.String() == "127.0.0.1" {
			return nil
		}
	}
	certConfig.AltNames.IPs = append(certConfig.AltNames.IPs, net.ParseIP("127.0.0.1"))
	return nil
}

func removeDuplicateIPs(ips []net.IP) []net.IP {
	m := make(map[string]bool)
	var out []net.IP
	for _, ip := range ips {
		s := ip.String()
		if m[s] {
			continue
		}
		m[s] = true
		out = append(out, ip)
	}
	return out
}
