/*
Copyright 2022 The Kubernetes Authors.

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

package execinitsystem

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"k8s.io/klog/v2"
	"sigs.k8s.io/etcdadm/apis"
)

const defaultEtcdStartupTimeout = 30 * time.Second
const defaultEtcdStopTimeout = 30 * time.Second

// New creates a new exec init system
func New(config *apis.EtcdAdmConfig) *ExecInitSystem {
	return &ExecInitSystem{
		desiredConfig: config,
	}
}

// ExecInitSystem runs etcd directly
type ExecInitSystem struct {
	mutex         sync.Mutex
	desiredConfig *apis.EtcdAdmConfig
	process       *process
}

// EnableAndStartService enables and starts the etcd service
func (s *ExecInitSystem) EnableAndStartService() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	cfg := s.desiredConfig

	// name := s.name(cfg)

	// We  precreate the data dir so we can use it as the working directory
	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		return fmt.Errorf("failed to create %s: %w", cfg.DataDir, err)
	}

	// TODO: Shutdown existing process?

	process, err := startProcess(cfg)
	if err != nil {
		return err
	}

	s.process = process

	return nil
}

// DisableAndStopService disables and stops the etcd service
func (s *ExecInitSystem) DisableAndStopService() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.process == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultEtcdStopTimeout)
	defer cancel()

	if err := s.process.Stop(ctx); err != nil {
		klog.Warningf("failed to stop etcd process: %v", err)
		return err
	}

	s.process = nil

	return nil
}

// IsActive checks if the systemd unit is active
func (s *ExecInitSystem) IsActive() (bool, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.process == nil {
		return false, nil
	}

	return s.process.IsActive()
}

// // SetConfiguration sets the desired etcd configuration
// func (s *ExecInitSystem) SetConfiguration(cfg *apis.EtcdAdmConfig) error {
// 	s.mutex.Lock()
// 	defer s.mutex.Unlock()

// 	// TODO: stop process?

// 	s.desiredConfig = cfg
// 	return nil
// }

// Install downloads all the necessary components
func (s *ExecInitSystem) Install() error {
	// TODO: download etcd
	return nil
}

// Configure boostraps the necessary configuration files for the etcd service
func (s *ExecInitSystem) Configure() error {
	return nil
}

// StartupTimeout defines the max time that the system should wait for etcd to be up
func (s *ExecInitSystem) StartupTimeout() time.Duration {
	return defaultEtcdStartupTimeout
}
