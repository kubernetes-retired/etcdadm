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
	"fmt"
	"net/url"
	"os"

	log "sigs.k8s.io/etcdadm/pkg/logrus"

	"go.etcd.io/etcd/api/v3/etcdserverpb"

	"sigs.k8s.io/etcdadm/apis"
	"sigs.k8s.io/etcdadm/constants"
	"sigs.k8s.io/etcdadm/etcd"
	"sigs.k8s.io/etcdadm/initsystem"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
)

func init() {
	runner := newJoinRunner()
	joinCmd := newJoinCmd(runner)
	rootCmd.AddCommand(joinCmd)
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.Name, "name", "", "etcd member name")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.Version, "version", constants.DefaultVersion, "etcd version")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.ReleaseURL, "release-url", constants.DefaultReleaseURL, "URL used to download etcd")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.CertificatesDir, "certs-dir", constants.DefaultCertificateDir, "certificates directory")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.BindAddr, "bind-address", "", "etcd bind address")
	joinCmd.PersistentFlags().StringSliceVar(&etcdAdmConfig.ServerCertSANs, "server-cert-extra-sans", etcdAdmConfig.ServerCertSANs, "optional extra Subject Alternative Names for the etcd server signing cert, can be multiple comma separated DNS names or IPs")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.InstallDir, "install-dir", constants.DefaultInstallDir, "install directory")
	joinCmd.PersistentFlags().StringArrayVar(&etcdAdmConfig.EtcdDiskPriorities, "disk-priorities", constants.DefaultEtcdDiskPriorities, "Setting etcd disk priority")
	joinCmd.PersistentFlags().BoolVar(&etcdAdmConfig.Retry, "retry", true, "Enable or disable backoff retry when join etcd member to cluster")
	joinCmd.PersistentFlags().StringVar((*string)(&etcdAdmConfig.InitSystem), "init-system", string(apis.Systemd), "init system type (systemd or kubelet)")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.ImageRepository, "image-repository", constants.DefaultImageRepository, "image repository when using kubelet init system")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.DataDir, "data-dir", constants.DefaultDataDir, "etcd data directory")
	joinCmd.PersistentFlags().StringVar(&etcdAdmConfig.PodSpecDir, "kubelet-pod-manifest-path", constants.DefaultPodSpecDir, "kubelet podspec directory")

	runner.registerPhasesAsSubcommands(joinCmd)
}

func newJoinCmd(runner *runner) *cobra.Command {
	return &cobra.Command{
		Use:   "join",
		Short: "Join an existing etcd cluster",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := runner.run(cmd, args); err != nil {
				log.Fatal(err)
			}
		},
	}
}

func newJoinRunner() *runner {
	runner := newRunner(joinPhasesSetup)
	runner.registerPhases(
		stop(),
		certificates(),
		membership(),
		joinInstall(),
		joinConfigure(),
		joinStart(),
		etcdctl(),
		healthcheck(),
	)

	return runner
}

func joinPhasesSetup(cmd *cobra.Command, args []string) (*phaseInput, error) {
	endpoint := args[0]
	if _, err := url.Parse(endpoint); err != nil {
		return nil, fmt.Errorf("endpoint %q must be a valid URL: %s", endpoint, err)
	}
	etcdAdmConfig.Endpoint = endpoint

	apis.SetDefaults(&etcdAdmConfig)
	if err := apis.SetJoinDynamicDefaults(&etcdAdmConfig); err != nil {
		return nil, fmt.Errorf("[defaults] error setting join dynamic defaults: %w", err)
	}

	initSystem, err := initsystem.GetInitSystem(&etcdAdmConfig)
	if err != nil {
		return nil, fmt.Errorf("[initsystem] error detecting the init system: %w", err)
	}

	in := &phaseInput{
		initSystem:    initSystem,
		etcdAdmConfig: &etcdAdmConfig,
	}

	return in, nil
}

func stop() phase {
	return &singlePhase{
		phaseName: "stop",
		runFunc: func(in *phaseInput) error {
			if err := in.initSystem.DisableAndStopService(); err != nil {
				return fmt.Errorf("error disabling and stopping etcd service: %w", err)
			}

			return nil
		},
	}
}

func membership() phase {
	return &singlePhase{
		phaseName: "membership",
		runFunc: func(in *phaseInput) error {
			if in.etcdAdmConfig.InitialCluster != "" {
				// we already run this phase
				return nil
			}

			var members []*etcdserverpb.Member
			log.Println("[membership] Checking if this member was added")
			client, err := etcd.ClientForEndpoint(in.etcdAdmConfig.Endpoint, in.etcdAdmConfig)
			if err != nil {
				return fmt.Errorf("error checking membership: %v", err)
			}
			defer client.Close()

			ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultEtcdRequestTimeout)
			mresp, err := client.MemberList(ctx)
			cancel()
			if err != nil {
				return fmt.Errorf("error listing members: %v", err)
			}
			members = mresp.Members
			localMember, ok := etcd.MemberForPeerURLs(members, in.etcdAdmConfig.InitialAdvertisePeerURLs.StringSlice())
			if !ok {
				log.Printf("[membership] Member was not added")
				log.Printf("Removing existing data dir %q", in.etcdAdmConfig.DataDir)
				os.RemoveAll(in.etcdAdmConfig.DataDir)
				log.Println("[membership] Adding member")

				var lastErr error
				retrySteps := 1
				if in.etcdAdmConfig.Retry {
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
					mresp, err := client.MemberAdd(ctx, in.etcdAdmConfig.InitialAdvertisePeerURLs.StringSlice())
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
					return fmt.Errorf("error adding member: %v", lastErr)
				}
			} else {
				log.Println("[membership] Member was added")
			}

			log.Println("[membership] Checking if member was started")
			if !etcd.Started(localMember) {
				log.Println("[membership] Member was not started")
				log.Printf("[membership] Removing existing data dir %q", in.etcdAdmConfig.DataDir)
				os.RemoveAll(in.etcdAdmConfig.DataDir)

				// To derive the initial cluster string, add the name and peerURLs to the local member
				localMember.Name = in.etcdAdmConfig.Name
				localMember.PeerURLs = in.etcdAdmConfig.InitialAdvertisePeerURLs.StringSlice()

				var desiredMembers []*etcdserverpb.Member
				for _, m := range members {
					if m.ID == localMember.ID {
						continue
					}
					desiredMembers = append(desiredMembers, m)
				}
				desiredMembers = append(desiredMembers, localMember)
				in.etcdAdmConfig.InitialCluster = etcd.InitialClusterFromMembers(desiredMembers)
			} else {
				log.Println("[membership] Member was started")
				log.Printf("[membership] Keeping existing data dir %q", in.etcdAdmConfig.DataDir)
				in.etcdAdmConfig.InitialCluster = etcd.InitialClusterFromMembers(members)
			}

			return nil
		},
	}
}

func joinInstall() phase {
	return &singlePhase{
		phaseName: "install",
		runFunc: func(in *phaseInput) error {
			if err := in.initSystem.Install(); err != nil {
				return fmt.Errorf("failed installing init system components: %w", err)
			}

			return nil
		},
	}
}

func joinConfigure() phase {
	return &singlePhase{
		phaseName:     "configure",
		prerequisites: []phase{membership()},
		runFunc: func(in *phaseInput) error {
			if err := in.initSystem.Configure(); err != nil {
				return fmt.Errorf("error configuring init system: %w", err)
			}

			return nil
		},
	}
}

func joinStart() phase {
	return &singlePhase{
		phaseName:     "start",
		prerequisites: []phase{membership()},
		runFunc: func(in *phaseInput) error {
			if err := in.initSystem.EnableAndStartService(); err != nil {
				return fmt.Errorf("error starting etcd: %w", err)
			}

			return nil
		},
	}
}
