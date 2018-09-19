package cmd

import (
	"context"
	"log"
	"net/url"
	"os"

	"github.com/coreos/etcd/etcdserver/etcdserverpb"

	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	"github.com/platform9/etcdadm/apis"
	"github.com/platform9/etcdadm/binary"
	"github.com/platform9/etcdadm/certs"
	"github.com/platform9/etcdadm/constants"
	"github.com/platform9/etcdadm/etcd"
	"github.com/platform9/etcdadm/service"

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

		apis.SetDefaults(&etcdAdmConfig)
		if err := apis.SetJoinDynamicDefaults(&etcdAdmConfig); err != nil {
			log.Fatalf("[defaults] Error: %s", err)
		}

		active, err := service.Active(constants.UnitFileBaseName)
		if err != nil {
			log.Fatalf("[start] Error checking if etcd service is active: %s", err)
		}
		if active {
			if err := service.Stop(constants.UnitFileBaseName); err != nil {
				log.Fatalf("[start] Error stopping existing etcd service: %s", err)
			}
		}

		enabled, err := service.Enabled(constants.UnitFileBaseName)
		if err != nil {
			log.Fatalf("[start] Error checking if etcd service is enabled: %s", err)
		}
		if enabled {
			if err := service.Disable(constants.UnitFileBaseName); err != nil {
				log.Fatalf("[start] Error disabling existing etcd service: %s", err)
			}
		}

		// cert management
		if err := certs.CreatePKIAssets(&etcdAdmConfig); err != nil {
			log.Fatalf("[certificates] Error: %s", err)
		}

		var localMember *etcdserverpb.Member
		var members []*etcdserverpb.Member
		log.Println("[membership] Checking if this member was added")
		client, err := etcd.ClientForEndpoint(endpoint, &etcdAdmConfig)
		if err != nil {
			log.Fatalf("[membership] Error checking membership: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultEtcdRequestTimeout)
		mresp, err := client.MemberList(ctx)
		cancel()
		if err != nil {
			log.Fatalf("[membership] Error listing members: %v", err)
		}
		members = mresp.Members
		localMember, ok := etcd.MemberForPeerURLs(members, etcdAdmConfig.InitialAdvertisePeerURLs.StringSlice())
		if !ok {
			log.Printf("[membership] Member was not added")
			log.Printf("Removing existing data dir %q", etcdAdmConfig.DataDir)
			os.RemoveAll(etcdAdmConfig.DataDir)
			log.Println("[membership] Adding member")
			ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultEtcdRequestTimeout)
			mresp, err := client.MemberAdd(ctx, etcdAdmConfig.InitialAdvertisePeerURLs.StringSlice())
			if err != nil {
				log.Fatalf("[membership] Error adding member: %v", err)
			}
			localMember = mresp.Member
			members = mresp.Members
			cancel()
		} else {
			log.Println("[membership] Member was added")
		}

		log.Println("[membership] Checking if member was started")
		if !etcd.Started(localMember) {
			log.Println("[membership] Member was not started")
			log.Printf("[membership] Removing existing data dir %q", etcdAdmConfig.DataDir)
			os.RemoveAll(etcdAdmConfig.DataDir)

			// To derive the initial cluster string, add the name and peerURLs to the local member
			localMember.Name = etcdAdmConfig.Name
			localMember.PeerURLs = etcdAdmConfig.InitialAdvertisePeerURLs.StringSlice()

			var desiredMembers []*etcdserverpb.Member
			for _, m := range members {
				if m.ID == localMember.ID {
					continue
				}
				desiredMembers = append(desiredMembers, m)
			}
			desiredMembers = append(desiredMembers, localMember)
			etcdAdmConfig.InitialCluster = etcd.InitialClusterFromMembers(desiredMembers)
		} else {
			log.Println("[membership] Member was started")
			log.Printf("[membership] Keeping existing data dir %q", etcdAdmConfig.DataDir)
			etcdAdmConfig.InitialCluster = etcd.InitialClusterFromMembers(members)
		}

		// etcd binaries installation
		inCache, err := binary.InstallFromCache(etcdAdmConfig.Version, etcdAdmConfig.InstallDir, etcdAdmConfig.CacheDir)
		if err != nil {
			log.Fatalf("[install] Artifact could not be installed from cache: %s", err)
		}
		if !inCache {
			log.Printf("[install] Artifact not found in cache. Trying to fetch from upstream: %s", err)
			if err = binary.Download(etcdAdmConfig.ReleaseURL, etcdAdmConfig.Version, etcdAdmConfig.CacheDir); err != nil {
				log.Fatalf("[install] Unable to fetch artifact from upstream: %s", err)
			}
			// Try installing binaries from cache now
			inCache, err := binary.InstallFromCache(etcdAdmConfig.Version, etcdAdmConfig.InstallDir, etcdAdmConfig.CacheDir)
			if err != nil {
				log.Fatalf("[install] Artifact could not be installed from cache: %s", err)
			}
			if !inCache {
				log.Fatalf("[install] Artifact not found in cache after download. Exiting.")
			}
		}
		installed, err := binary.IsInstalled(etcdAdmConfig.Version, etcdAdmConfig.InstallDir)
		if err != nil {
			log.Fatalf("[install] Error: %s", err)
		}
		if !installed {
			log.Fatalf("[install] Binaries not found in install dir. Exiting.")
		}

		if err := service.WriteEnvironmentFile(&etcdAdmConfig); err != nil {
			log.Fatalf("[configure] Error: %s", err)
		}
		if err := service.WriteUnitFile(&etcdAdmConfig); err != nil {
			log.Fatalf("[configure] Error: %s", err)
		}
		if err := service.EnableAndStartService(constants.UnitFileBaseName); err != nil {
			log.Fatalf("[start] Error: %s", err)
		}
		if err := service.WriteEtcdctlEnvFile(&etcdAdmConfig); err != nil {
			log.Printf("[configure] Warning: %s", err)
		}
		if err := service.WriteEtcdctlShellWrapper(&etcdAdmConfig); err != nil {
			log.Printf("[configure] Warning: %s", err)
		}

		log.Println("[health] Checking local etcd endpoint health")
		client, err = etcd.ClientForEndpoint(etcdAdmConfig.LoopbackClientURL.String(), &etcdAdmConfig)
		if err != nil {
			log.Printf("[health] Error checking health: %v", err)
		}
		ctx, cancel = context.WithTimeout(context.Background(), constants.DefaultEtcdRequestTimeout)
		_, err = client.Get(ctx, constants.EtcdHealthCheckKey)
		cancel()
		// Healthy because the cluster reaches consensus for the get request,
		// even if permission (to get the key) is denied.
		if err == nil || err == rpctypes.ErrPermissionDenied {
			log.Println("[health] Local etcd endpoint is healthy")
		} else {
			log.Fatalf("[health] Local etcd endpoint is unhealthy: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(joinCmd)
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.Name, "name", "", "etcd member name")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.Version, "version", constants.DefaultVersion, "etcd version")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.ReleaseURL, "release-url", constants.DefaultReleaseURL, "URL used to download etcd")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.CertificatesDir, "certs-dir", constants.DefaultCertificateDir, "certificates directory")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.InstallDir, "install-dir", constants.DefaultInstallDir, "install directory")
}
