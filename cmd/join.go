/*
Copyright 2018 The Kubernetes Authors.

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

package cmd

import (
	"context"
	"net/url"
	"os"

	log "sigs.k8s.io/etcdadm/pkg/logrus"

	"github.com/coreos/etcd/etcdserver/etcdserverpb"

	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	"sigs.k8s.io/etcdadm/apis"
	"sigs.k8s.io/etcdadm/binary"
	"sigs.k8s.io/etcdadm/certs"
	"sigs.k8s.io/etcdadm/constants"
	"sigs.k8s.io/etcdadm/etcd"
	"sigs.k8s.io/etcdadm/initsystem"
	"sigs.k8s.io/etcdadm/service"
	"sigs.k8s.io/etcdadm/util"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
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

		initSystem, err := initsystem.GetInitSystem()
		if err != nil {
			log.Fatalf("[initsystem] Error detecting the init system: %s", err)
		}

		active, err := initSystem.IsActive(constants.UnitFileBaseName)
		if err != nil {
			log.Fatalf("[start] Error checking if etcd service is active: %s", err)
		}
		if active {
			if err := initSystem.Stop(constants.UnitFileBaseName); err != nil {
				log.Fatalf("[start] Error stopping existing etcd service: %s", err)
			}
		}

		enabled, err := initSystem.IsEnabled(constants.UnitFileBaseName)
		if err != nil {
			log.Fatalf("[start] Error checking if etcd service is enabled: %s", err)
		}
		if enabled {
			if err := initSystem.Disable(constants.UnitFileBaseName); err != nil {
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

			var lastErr error
			retrySteps := 1
			if etcdAdmConfig.Retry {
				retrySteps = constants.DefaultBackOffSteps
			}

			// Exponential backoff for MemberAdd (values exclude jitter):
			// If --retry=false, add member only try one times, otherwise try five times.
			// The backoff duration is 0, 2, 4, 8, 16 s
			opts := wait.Backoff{
				Duration: constants.DefaultBackOffDuration,
				Steps:    retrySteps,
				Factor:   constants.DefaultBackOffFactor,
				Jitter:   0.1,
			}
			err := wait.ExponentialBackoff(opts, func() (bool, error) {
				ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultEtcdRequestTimeout)
				mresp, err := client.MemberAdd(ctx, etcdAdmConfig.InitialAdvertisePeerURLs.StringSlice())
				cancel()
				if err != nil {
					log.Warningf("[membership] Error adding member: %v, will retry after %s.", err, opts.Step())
					lastErr = err
					return false, nil
				}
				localMember = mresp.Member
				members = mresp.Members
				return true, nil
			})
			if err != nil {
				log.Fatalf("[membership] Error adding member: %v", lastErr)
			}
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
			log.Printf("[install] Artifact not found in cache. Trying to fetch from upstream: %s", etcdAdmConfig.ReleaseURL)
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
		if err := initSystem.EnableAndStartService(constants.UnitFileBaseName); err != nil {
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
	joinCmd.PersistentFlags().StringSliceVar(&etcdAdmConfig.ServerCertSANs, "server-cert-extra-sans", etcdAdmConfig.ServerCertSANs, "optional extra Subject Alternative Names for the etcd server signing cert, can be multiple comma separated DNS names or IPs")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.InstallDir, "install-dir", constants.DefaultInstallDir, "install directory")
	joinCmd.PersistentFlags().StringArrayVar(&etcdAdmConfig.EtcdDiskPriorities, "disk-priorities", constants.DefaultEtcdDiskPriorities, "Setting etcd disk priority")
	joinCmd.PersistentFlags().BoolVar(&etcdAdmConfig.Retry, "retry", true, "Enable or disable backoff retry when join etcd member to cluster")
	joinCmd.PersistentFlags().Var(util.URLValue{URL: &etcdAdmConfig.AdditionalMetricsURL}, "listen-metrics-url", "Additional URL to expose /metrics endpoint without TLS")
}
