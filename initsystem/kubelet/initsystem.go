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

package kubelet

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"time"

	"k8s.io/klog"
	"sigs.k8s.io/etcdadm/apis"
	"sigs.k8s.io/etcdadm/service"
	"sigs.k8s.io/yaml"
)

const DefaultEtcdStartupTimeout = 30 * time.Second

func New(config *apis.EtcdAdmConfig) *InitSystem {
	return &InitSystem{
		desiredConfig: config,
	}
}

// InitSystem runs etcd under the kubelet
type InitSystem struct {
	desiredConfig *apis.EtcdAdmConfig
}

// EnableAndStartService enables and starts the etcd service
func (s *InitSystem) EnableAndStartService(service string) error {
	cfg := s.desiredConfig

	name := s.name(cfg)

	// We have to precreate the data dir for kubelet
	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		return fmt.Errorf("failed to create %s: %w", cfg.DataDir, err)
	}

	pod, err := s.buildPod(name, cfg)
	if err != nil {
		return err
	}

	podBytes, err := yaml.Marshal(pod)
	if err != nil {
		return fmt.Errorf("failed to build pod yaml: %w", err)
	}

	podFile := s.podFile(cfg)
	klog.Infof("writing manifest at %s", podFile)
	if err := ioutil.WriteFile(podFile, podBytes, 0644); err != nil {
		return fmt.Errorf("unable to write %q: %w", podFile, err)
	}

	// TODO: Wait for etcd to start?

	return nil
}

// DisableAndStopService disables and stops the etcd service
func (s *InitSystem) DisableAndStopService(service string) error {
	podFile := s.podFile(s.desiredConfig)
	if err := os.Remove(podFile); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("error stopping etcd: %w", err)
	}

	// TODO: Wait for pod to stop?
	return nil
}

// IsActive checks if the systemd unit is active
func (s *InitSystem) IsActive(service string) (bool, error) {
	podFile := s.podFile(s.desiredConfig)
	_, err := os.Stat(podFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("error checking for etcd manifest: %w", err)
	}
	return true, nil
}

func (s *InitSystem) podFile(cfg *apis.EtcdAdmConfig) string {
	name := s.name(cfg)
	return "/etc/kubernetes/manifests/" + name + ".manifest"
}

func (s *InitSystem) name(cfg *apis.EtcdAdmConfig) string {
	return "etcd" // TODO: Main and events
}

// SetConfiguration sets the desired etcd configuration
func (s *InitSystem) SetConfiguration(cfg *apis.EtcdAdmConfig) error {
	s.desiredConfig = cfg
	return nil
}

func (s *InitSystem) buildPod(name string, cfg *apis.EtcdAdmConfig) (*pod, error) {
	env := make(map[string]string)

	// TODO: This is not the best way to build the environment, but it lets us minimize the initial changes
	{
		envBytes, err := service.BuildEnvironment(cfg)
		if err != nil {
			return nil, err
		}

		for _, line := range strings.Split(string(envBytes), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			tokens := strings.SplitN(line, "=", 2)
			if len(tokens) != 2 {
				return nil, fmt.Errorf("cannot parse environment line %q", line)
			}

			env[tokens[0]] = tokens[1]
		}
	}

	// TODO: Critical pod annotations
	// TODO: Should we use the full/official kubernetes API libraries?

	pod := &pod{
		APIVersion: "v1",
		Kind:       "Pod",
	}
	pod.Metadata = objectMeta{
		Labels: map[string]string{
			"k8s-app": name,
		},
		Name:      name,
		Namespace: "kube-system",
	}

	podSpec := &pod.Spec

	container := container{
		Name: "etcd",
	}

	container.SecurityContext.Privileged = true
	container.Resources.Requests = map[string]string{
		"cpu":    "100m",
		"memory": "100Mi",
	}

	container.Image = s.image()
	container.Command = []string{"etcd"}

	for k, v := range env {
		container.Env = append(container.Env, containerEnv{
			Name:  k,
			Value: v,
		})
	}

	sort.Slice(container.Env, func(i, j int) bool {
		return container.Env[i].Name < container.Env[j].Name
	})

	{
		mount := volumeMount{
			Name:      "varlogetcd",
			MountPath: "/var/log/etcd.log",
		}
		volume := podVolume{
			Name: mount.Name,
			HostPath: &hostPath{
				Path: "/var/log/" + name + ".log",
				Type: "FileOrCreate",
			},
		}
		container.VolumeMounts = append(container.VolumeMounts, mount)
		podSpec.Volumes = append(podSpec.Volumes, volume)
	}

	{
		mount := volumeMount{
			Name:      "pki",
			MountPath: cfg.CertificatesDir,
		}
		volume := podVolume{
			Name: mount.Name,
			HostPath: &hostPath{
				Path: cfg.CertificatesDir,
				Type: "Directory",
			},
		}
		container.VolumeMounts = append(container.VolumeMounts, mount)
		podSpec.Volumes = append(podSpec.Volumes, volume)
	}

	{
		mount := volumeMount{
			Name:      "data",
			MountPath: cfg.DataDir,
		}
		volume := podVolume{
			Name: mount.Name,
			HostPath: &hostPath{
				Path: cfg.DataDir,
				Type: "Directory",
			},
		}
		container.VolumeMounts = append(container.VolumeMounts, mount)
		podSpec.Volumes = append(podSpec.Volumes, volume)
	}

	podSpec.Containers = append(podSpec.Containers, container)
	podSpec.HostNetwork = true
	podSpec.PriorityClassName = "system-cluster-critical"

	return pod, nil
}

func (s *InitSystem) image() string {
	return fmt.Sprintf("%s:v%s", s.desiredConfig.ImageRepository, s.desiredConfig.Version)
}

// pod (and the other types) are a minimal version of the k8s Pod API
// This avoids having to pull in all of the kubernetes API and infrastructure
type pod struct {
	APIVersion string     `json:"apiVersion"`
	Kind       string     `json:"kind"`
	Metadata   objectMeta `json:"metadata"`
	Spec       podSpec    `json:"spec"`
}

type objectMeta struct {
	Labels    map[string]string `json:"labels"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
}

type podSpec struct {
	Containers        []container `json:"containers"`
	HostNetwork       bool        `json:"hostNetwork"`
	PriorityClassName string      `json:"priorityClassName"`
	Volumes           []podVolume `json:"volumes"`
}

type container struct {
	Command         []string           `json:"command"`
	Env             []containerEnv     `json:"env"`
	Image           string             `json:"image"`
	Name            string             `json:"name"`
	Resources       containerResources `json:"resources"`
	SecurityContext securityContext    `json:"securityContext"`
	VolumeMounts    []volumeMount      `json:"volumeMounts"`
}

type containerEnv struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type containerResources struct {
	Requests map[string]string `json:"requests,omitempty"`
	Limits   map[string]string `json:"limits,omitempty"`
}

type securityContext struct {
	Privileged bool `json:"privileged"`
}

type volumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath,omitempty"`
}

type podVolume struct {
	Name     string    `json:"name"`
	HostPath *hostPath `json:"hostPath,omitempty"`
}

type hostPath struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

func (s InitSystem) Install() error {
	// TODO: preload etcd image to make start up times more reliable?
	return nil
}

// Configure boostraps the necessary configuration files for the etcd service
func (s InitSystem) Configure() error {
	// TODO: move manifest generation here?
	return nil
}

// StartupTimeout defines the max time that the system should wait for etcd to be up
func (s InitSystem) StartupTimeout() time.Duration {
	return DefaultEtcdStartupTimeout
}
