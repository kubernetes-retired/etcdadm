package cmd

import (
	"context"
	"log"
	"os"

	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	"github.com/platform9/etcdadm/etcd"

	"github.com/platform9/etcdadm/preflight"

	"github.com/platform9/etcdadm/apis"
	"github.com/platform9/etcdadm/binary"
	"github.com/platform9/etcdadm/certs"
	"github.com/platform9/etcdadm/constants"
	"github.com/platform9/etcdadm/util"

	"github.com/platform9/etcdadm/service"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new etcd cluster",
	Run: func(cmd *cobra.Command, args []string) {
		apis.SetDefaults(&etcdAdmConfig)
		if err := apis.SetInitDynamicDefaults(&etcdAdmConfig); err != nil {
			log.Fatalf("[defaults] Error: %s", err)
		}

		log.Println("[pre-flight] Running mandatory checks")
		if err := preflight.Mandatory(&etcdAdmConfig); err != nil {
			log.Fatalf("[pre-flight] Error: %v", err)
		}
		log.Println("[pre-flight] Passed mandatory checks.")

		service.DisableAndStopService(constants.UnitFileBaseName)
		exists, err := util.Exists(etcdAdmConfig.DataDir)
		if err != nil {
			log.Fatalf("Unable to verify whether data dir exists: %v", err)
		}
		if exists {
			log.Printf("[install] Removing existing data dir %q", etcdAdmConfig.DataDir)
			os.RemoveAll(etcdAdmConfig.DataDir)
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
		// cert management
		if err = certs.CreatePKIAssets(&etcdAdmConfig); err != nil {
			log.Fatalf("[certificates] Error: %s", err)
		}
		if etcdAdmConfig.Snapshot != "" {
			if err := util.RestoreSnapshot(&etcdAdmConfig); err != nil {
				log.Fatalf("[snapshot] Error restoring snapshot: %v", err)
			}
		}
		if err = service.WriteEnvironmentFile(&etcdAdmConfig); err != nil {
			log.Fatalf("[configure] Error: %s", err)
		}
		if err = service.WriteUnitFile(&etcdAdmConfig); err != nil {
			log.Fatalf("[configure] Error: %s", err)
		}
		if err = service.EnableAndStartService(constants.UnitFileBaseName); err != nil {
			log.Fatalf("[start] Error: %s", err)
		}
		if err = service.WriteEtcdctlEnvFile(&etcdAdmConfig); err != nil {
			log.Printf("[configure] Warning: %s", err)
		}
		if err = service.WriteEtcdctlShellWrapper(&etcdAdmConfig); err != nil {
			log.Printf("[configure] Warning: %s", err)
		}

		log.Println("[health] Checking local etcd endpoint health")
		client, err := etcd.ClientForEndpoint(etcdAdmConfig.LoopbackClientURL.String(), &etcdAdmConfig)
		if err != nil {
			log.Printf("[health] Error checking health: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultEtcdRequestTimeout)
		_, err = client.Get(ctx, constants.EtcdHealthCheckKey)
		cancel()
		// Healthy because the cluster reaches consensus for the get request,
		// even if permission (to get the key) is denied.
		if err == nil || err == rpctypes.ErrPermissionDenied {
			log.Println("[health] Local etcd endpoint is healthy")
		} else {
			log.Fatalf("[health] Local etcd endpoint is unhealthy: %v", err)
		}

		// Output etcdadm join command
		// TODO print all advertised client URLs (first, join must parse than one endpoint)
		log.Println("To add another member to the cluster, copy the CA cert/key to its certificate dir and run:")
		log.Printf(`	etcdadm join %s`, etcdAdmConfig.AdvertiseClientURLs[0].String())
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.Name, "name", "", "etcd member name")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.Version, "version", constants.DefaultVersion, "etcd version")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.ReleaseURL, "release-url", constants.DefaultReleaseURL, "URL used to download etcd")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.CertificatesDir, "certs-dir", constants.DefaultCertificateDir, "certificates directory")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.InstallDir, "install-dir", constants.DefaultInstallDir, "install directory")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.Snapshot, "snapshot", "", "Etcd v3 snapshot file used to initialize member")
	initCmd.PersistentFlags().BoolVar(&etcdAdmConfig.SkipHashCheck, "skip-hash-check", false, "Ignore snapshot integrity hash value (required if copied from data directory)")
	initCmd.PersistentFlags().DurationVar(&etcdAdmConfig.DownloadConnectTimeout, "download-connect-timeout", constants.DefaultDownloadConnectTimeout, "Maximum time in seconds that you allow the connection to the server to take.")
}
