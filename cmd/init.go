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
	"os"

	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	"github.com/spf13/cobra"

	"sigs.k8s.io/etcdadm/apis"
	"sigs.k8s.io/etcdadm/certs"
	"sigs.k8s.io/etcdadm/constants"
	"sigs.k8s.io/etcdadm/etcd"
	"sigs.k8s.io/etcdadm/initsystem"
	log "sigs.k8s.io/etcdadm/pkg/logrus"
	"sigs.k8s.io/etcdadm/service"
	"sigs.k8s.io/etcdadm/util"
)

func init() {
	runner := newInitRunner()
	initCmd := newInitCmd(runner)
	rootCmd.AddCommand(initCmd)
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.Name, "name", "", "etcd member name")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.Version, "version", constants.DefaultVersion, "etcd version")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.ReleaseURL, "release-url", constants.DefaultReleaseURL, "URL used to download etcd")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.CertificatesDir, "certs-dir", constants.DefaultCertificateDir, "certificates directory")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.BindAddr, "bind-address", "", "etcd bind address")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.UnitFile, "unit-file", "/etc/systemd/system/etcd.service", "system unit file")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.EnvironmentFile, "env-file", "/etc/etcd/etcd.env", "system env file")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.EtcdctlEnvFile, "ctl-file", "/etc/etcd/etcdctl.env", "system etcd ctl file")
	initCmd.PersistentFlags().IntVar(&etcdAdmConfig.ClientPort, "client-port", 2379, "etcd port listening from client")
	initCmd.PersistentFlags().IntVar(&etcdAdmConfig.CertValid, "cert-valid", 1, "cert validate time. (by year)")
	initCmd.PersistentFlags().IntVar(&etcdAdmConfig.PeerPort, "peer-port", 2380, "etcd port listening from peer etcd instance")
	initCmd.PersistentFlags().StringSliceVar(&etcdAdmConfig.ServerCertSANs, "server-cert-extra-sans", etcdAdmConfig.ServerCertSANs, "optional extra Subject Alternative Names for the etcd server signing cert, can be multiple comma separated DNS names or IPs")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.InstallDir, "install-dir", constants.DefaultInstallDir, "install directory")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.Snapshot, "snapshot", "", "Etcd v3 snapshot file used to initialize member")
	initCmd.PersistentFlags().BoolVar(&etcdAdmConfig.SkipHashCheck, "skip-hash-check", false, "Ignore snapshot integrity hash value (required if copied from data directory)")
	initCmd.PersistentFlags().DurationVar(&etcdAdmConfig.DownloadConnectTimeout, "download-connect-timeout", constants.DefaultDownloadConnectTimeout, "Maximum time in seconds that you allow the connection to the server to take.")
	initCmd.PersistentFlags().StringArrayVar(&etcdAdmConfig.EtcdDiskPriorities, "disk-priorities", constants.DefaultEtcdDiskPriorities, "Setting etcd disk priority")
	initCmd.PersistentFlags().StringVar((*string)(&etcdAdmConfig.InitSystem), "init-system", string(apis.Systemd), "init system type (systemd or kubelet)")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.ImageRepository, "image-repository", constants.DefaultImageRepository, "image repository when using kubelet init system")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.DataDir, "data-dir", constants.DefaultDataDir, "etcd data directory")
	initCmd.PersistentFlags().StringVar(&etcdAdmConfig.PodSpecDir, "kubelet-pod-manifest-path", constants.DefaultPodSpecDir, "kubelet podspec directory")

	runner.registerPhasesAsSubcommands(initCmd)
}

func newInitCmd(runner *runner) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize a new etcd cluster",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runner.run(cmd, args); err != nil {
				log.Fatal(err)
			}
		},
	}
}

func newInitRunner() *runner {
	runner := newRunner(initPhasesSetup)
	runner.registerPhases(
		initInstall(),
		certificates(),
		snapshot(),
		initConfigure(),
		initStart(),
		etcdctl(),
		healthcheck(),
		postInitInstructions(),
	)

	return runner
}

func initPhasesSetup(_ *cobra.Command, _ []string) (*phaseInput, error) {
	apis.SetDefaults(&etcdAdmConfig)
	if err := apis.SetInitDynamicDefaults(&etcdAdmConfig); err != nil {
		return nil, fmt.Errorf("[defaults] error setting init dynamic defaults: %w", err)
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

func initInstall() phase {
	return &singlePhase{
		phaseName: "install",
		runFunc: func(in *phaseInput) error {
			if err := in.initSystem.DisableAndStopService(); err != nil {
				return fmt.Errorf("error disabling and stopping etcd service: %w", err)
			}
			exists, err := util.Exists(in.etcdAdmConfig.DataDir)
			if err != nil {
				return fmt.Errorf("unable to verify whether data dir exists: %w", err)
			}
			if exists {
				log.Printf("[install] Removing existing data dir %q", in.etcdAdmConfig.DataDir)
				os.RemoveAll(in.etcdAdmConfig.DataDir)
			}
			if err = in.initSystem.Install(); err != nil {
				return fmt.Errorf("failed installing init system components: %w", err)
			}

			return nil
		},
	}
}

func certificates() phase {
	return &singlePhase{
		phaseName: "certificates",
		runFunc: func(in *phaseInput) error {
			if err := certs.CreatePKIAssets(in.etcdAdmConfig); err != nil {
				return fmt.Errorf("failed creating PKI assets: %w", err)
			}

			return nil
		},
	}
}

func snapshot() phase {
	return &singlePhase{
		phaseName: "snapshot",
		runFunc: func(in *phaseInput) error {
			if in.etcdAdmConfig.Snapshot != "" {
				if err := etcd.RestoreSnapshot(&etcdAdmConfig); err != nil {
					return fmt.Errorf("error restoring snapshot: %w", err)
				}
			}

			return nil
		},
	}
}

func initConfigure() phase {
	return &singlePhase{
		phaseName: "configure",
		runFunc: func(in *phaseInput) error {
			if err := in.initSystem.Configure(); err != nil {
				return fmt.Errorf("error configuring init system: %w", err)
			}

			return nil
		},
	}
}

func initStart() phase {
	return &singlePhase{
		phaseName: "start",
		runFunc: func(in *phaseInput) error {
			if err := in.initSystem.EnableAndStartService(); err != nil {
				return fmt.Errorf("error starting etcd: %w", err)
			}

			return nil
		},
	}
}

func etcdctl() phase {
	return &singlePhase{
		phaseName: "etcdctl",
		runFunc: func(in *phaseInput) error {
			if err := service.WriteEtcdctlEnvFile(in.etcdAdmConfig); err != nil {
				log.Printf("[etcdctl] Warning: %s", err)
			}
			if err := service.WriteEtcdctlShellWrapper(in.etcdAdmConfig); err != nil {
				log.Printf("[etcdctl] Warning: %s", err)
			}

			return nil
		},
	}
}

func healthcheck() phase {
	return &singlePhase{
		phaseName: "health",
		runFunc: func(in *phaseInput) error {
			log.Println("[health] Checking local etcd endpoint health")
			client, err := etcd.ClientForEndpoint(in.etcdAdmConfig.LoopbackClientURL.String(), in.etcdAdmConfig)
			if err != nil {
				return fmt.Errorf("error creating health endpoint client: %w", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), in.initSystem.StartupTimeout())
			_, err = client.Get(ctx, constants.EtcdHealthCheckKey)
			cancel()
			// Healthy because the cluster reaches consensus for the get request,
			// even if permission (to get the key) is denied.
			if err == nil || err == rpctypes.ErrPermissionDenied {
				log.Println("[health] Local etcd endpoint is healthy")
			} else {
				return fmt.Errorf("local etcd endpoint is unhealthy: %w", err)
			}

			return nil
		},
	}
}

func postInitInstructions() phase {
	return &singlePhase{
		phaseName: "post-init-instructions",
		runFunc: func(in *phaseInput) error {
			// Output etcdadm join command
			// TODO print all advertised client URLs (first, join must parse than one endpoint)
			log.Println("To add another member to the cluster, copy the CA cert/key to its certificate dir and run:")
			log.Printf(`	etcdadm join %s`, etcdAdmConfig.AdvertiseClientURLs[0].String())

			return nil
		},
	}
}
