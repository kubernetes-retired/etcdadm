package cmd

import (
	"context"
	"log"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/platform9/etcdadm/apis"
	"github.com/platform9/etcdadm/binary"
	"github.com/platform9/etcdadm/certs"
	"github.com/platform9/etcdadm/constants"
	"github.com/platform9/etcdadm/service"

	"github.com/coreos/etcd/client"
	"github.com/spf13/cobra"
)

var joinCmd = &cobra.Command{
	Use:   "join",
	Short: "Join an existing etcd cluster",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		endpoint := args[0]
		if _, err := url.Parse(endpoint); err != nil {
			log.Fatalf("Error: endpoint %q must be a valid URL: %s", endpoint, err)
		}

		var err error

		err = apis.SetJoinDynamicDefaults(&etcdAdmConfig)
		if err != nil {
			log.Fatalf("[defaults] Error: %s", err)
		}
		err = binary.EnsureInstalled(etcdAdmConfig.ReleaseURL, etcdAdmConfig.Version, etcdAdmConfig.InstallDir)
		if err != nil {
			log.Fatalf("[install] Error: %s", err)
		}
		err = certs.CreatePKIAssets(&etcdAdmConfig)
		if err != nil {
			log.Fatalf("[certificates] Error: %s", err)
		}

		// Add self to cluster
		cfg := client.Config{
			Endpoints:               []string{endpoint},
			Transport:               client.DefaultTransport,
			HeaderTimeoutPerRequest: time.Second,
		}
		c, err := client.New(cfg)
		if err != nil {
			log.Fatal(err)
		}
		mapi := client.NewMembersAPI(c)
		peerURLs := strings.Split(etcdAdmConfig.AdvertisePeerURLs, ",")
		if len(peerURLs) == 0 {
			log.Fatalf("Error: cannot add member to cluster: no peer URL defined")
		}
		chosenPeerURL := peerURLs[0]
		_, err = mapi.Add(context.Background(), chosenPeerURL)
		if err != nil {
			log.Fatalf("[cluster] Error: failed to add member with peerURL %q to cluster: %s", chosenPeerURL, err)
		}
		log.Printf("[cluster] added member with peerURL %q to cluster", chosenPeerURL)
		// End

		err = service.WriteEnvironmentFile(&etcdAdmConfig)
		if err != nil {
			log.Fatalf("[configure] Error: %s", err)
		}
		err = service.WriteUnitFile(&etcdAdmConfig)
		if err != nil {
			log.Fatalf("[configure] Error: %s", err)
		}
		unit := filepath.Base(constants.UnitFile)
		err = service.EnableAndStartService(unit)
		if err != nil {
			log.Fatalf("[start] Error: %s", err)
		}
		err = service.WriteEtcdctlEnvFile(&etcdAdmConfig)
		if err != nil {
			log.Printf("[configure] Warning: %s", err)
		}

		// Verify cluster?
	},
}

func init() {
	rootCmd.AddCommand(joinCmd)
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.Name, "name", "", "etcd member name")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.InitialClusterToken, "token", "", "initial cluster token")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.Version, "version", constants.DefaultVersion, "etcd version")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.ReleaseURL, "release-url", constants.DefaultReleaseURL, "URL used to download etcd")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.CertificatesDir, "certs-dir", constants.DefaultCertificateDir, "certificates directory")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.InstallBaseDir, "install-base-dir", constants.DefaultInstallBaseDir, "install base directory")
}
