package etcd

import (
	"crypto/x509"
	"fmt"
	"net"
	"net/url"
	"path/filepath"

	"github.com/golang/glog"
	certutil "k8s.io/client-go/util/cert"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/pkg/pki"
)

func (p *etcdProcess) createKeypairs(peersCA *pki.Keypair, clientsCA *pki.Keypair, pkiDir string, me *protoetcd.EtcdNode) error {
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
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		}

		if err := addAltNames(&certConfig, me.PeerUrls); err != nil {
			return err
		}

		_, err := keypairs.EnsureKeypair("me", certConfig, peersCA)
		if err != nil {
			return err
		}
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

		certConfig := certutil.Config{
			CommonName: me.Name,
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		}

		if err := addAltNames(&certConfig, me.ClientUrls); err != nil {
			return err
		}

		glog.Infof("building client-serving certificate: %+v", certConfig)

		_, err := keypairs.EnsureKeypair("server", certConfig, clientsCA)
		if err != nil {
			return err
		}
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
		case "http":
			glog.Warningf("not generate certificate for http url %q", urlString)
		case "https":
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
	certConfig.AltNames.IPs = append(certConfig.AltNames.IPs, net.ParseIP("127.0.0.1"))

	return nil
}
