/**
 *   Copyright 2018 The etcdadm authors
 *
 *   Licensed under the Apache License, Version 2.0 (the "License");
 *   you may not use this file except in compliance with the License.
 *   You may obtain a copy of the License at
 *
 *       http://www.apache.org/licenses/LICENSE-2.0
 *
 *   Unless required by applicable law or agreed to in writing, software
 *   distributed under the License is distributed on an "AS IS" BASIS,
 *   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *   See the License for the specific language governing permissions and
 *   limitations under the License.
 */

package cmd

import (
	"context"
	"os"

	log "sigs.k8s.io/etcdadm/pkg/logrus"

	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	"sigs.k8s.io/etcdadm/etcd"

	"sigs.k8s.io/etcdadm/apis"
	"sigs.k8s.io/etcdadm/binary"
	"sigs.k8s.io/etcdadm/certs"
	"sigs.k8s.io/etcdadm/constants"
	"sigs.k8s.io/etcdadm/util"

	"sigs.k8s.io/etcdadm/service"
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
			log.Fatalf("[install] Error checking if etcd service is enabled: %s", err)
		}
		if enabled {
			if err := service.Disable(constants.UnitFileBaseName); err != nil {
				log.Fatalf("[install] Error disabling existing etcd service: %s", err)
			}
		}

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
			if err := etcd.RestoreSnapshot(&etcdAdmConfig); err != nil {
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
