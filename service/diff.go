/**
 *   Copyright 2018 Platform9 Systems, Inc.
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

package service

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/platform9/etcdadm/util"

	"github.com/platform9/etcdadm/apis"
	"github.com/platform9/etcdadm/constants"
)

var (
	etcdVersionRegexp    = regexp.MustCompile(`^etcd Version: (.*?)$`)
	etcdExecutableRegexp = regexp.MustCompile(`^ExecStart=(.*?)$`)
)

// DiffEnvironmentFile compares the desired configuration with the configuration
// previously written to disk and returns the difference, if any.
func DiffEnvironmentFile(cfg *apis.EtcdAdmConfig) (map[string]string, error) {
	diff := make(map[string]string)

	exists, lu, err := lastUsedEnvironment(cfg.EnvironmentFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read last used environment: %v", err)
	}
	if !exists {
		return diff, nil
	}

	d, err := desiredEnvironment(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to generate desired environment: %v", err)
	}

	for k, v := range d {
		if strings.Compare(v, lu[k]) != 0 {
			if strings.Compare(k, "ETCD_INITIAL_CLUSTER_TOKEN") == 0 {
				// The cluster token will never match, but mismatches can be ignored
				continue
			}
			diff[k] = fmt.Sprintf("desired %q, last used %q", v, lu[k])
		}
	}
	return diff, nil
}

// DiffVersion compares the desired etcd version with installed etcd version and
// returns the difference, if any.
func DiffVersion(cfg *apis.EtcdAdmConfig) (string, error) {
	exists, le, err := lastUsedEtcdExecutable(cfg.UnitFile)
	if err != nil {
		return "", fmt.Errorf("unable to find path of last used etcd executable: %v", err)
	}
	if !exists {
		return "", nil
	}
	exists, err = util.Exists(le)
	if err != nil {
		return "", fmt.Errorf("unable to check if %q exists: %v", le, err)
	}
	if !exists {
		return "", nil
	}

	cv := cfg.Version
	lv, err := etcdVersionForExecutable(le)
	if err != nil {
		return "", fmt.Errorf("unable to determine version: %v", err)
	}
	if strings.Compare(cv, lv) != 0 {
		return fmt.Sprintf("desired: %q, last used :%q", cv, lv), nil
	}
	return "", nil
}

func desiredEnvironment(cfg *apis.EtcdAdmConfig) (map[string]string, error) {
	t := template.Must(template.New("environment").Parse(constants.EnvFileTemplate))
	var b bytes.Buffer
	t.Execute(&b, cfg)
	return makeEnvironment(&b)
}

func lastUsedEnvironment(environmentFile string) (bool, map[string]string, error) {
	exists, err := util.Exists(environmentFile)
	if err != nil {
		return false, nil, fmt.Errorf("unable to check if %q exists: %v", environmentFile, err)
	}
	if !exists {
		return exists, nil, nil
	}

	f, err := os.Open(environmentFile)
	if err != nil {
		return exists, nil, err
	}
	defer f.Close()
	env, err := makeEnvironment(f)
	return exists, env, err
}

func makeEnvironment(r io.Reader) (map[string]string, error) {
	cfg := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		l := scanner.Text()
		if strings.HasPrefix(l, "#") {
			// Ignore comments
			continue
		}
		// Capture values with embedded '='
		kv := strings.SplitN(l, "=", 2)
		var k, v string
		k = strings.TrimSpace(kv[0])
		if len(kv) == 2 {
			v = strings.TrimSpace(kv[1])
		}
		cfg[k] = v
	}
	err := scanner.Err()
	return cfg, err
}

func lastUsedEtcdExecutable(unitFile string) (bool, string, error) {
	exists, err := util.Exists(unitFile)
	if err != nil {
		return false, "", fmt.Errorf("unable to check if %q exists: %v", unitFile, err)
	}
	if !exists {
		return exists, "", nil
	}

	f, err := os.Open(unitFile)
	if err != nil {
		return exists, "", err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		match := etcdExecutableRegexp.FindStringSubmatch(scanner.Text())
		if len(match) == 2 {
			return exists, match[1], nil
		}
	}
	return exists, "", fmt.Errorf("unable to parse executable from systemd unit %q", constants.UnitFile)
}

func etcdVersionForExecutable(etcdExecutable string) (string, error) {
	cmd := exec.Command(etcdExecutable, "--version")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("unable to check etcd version: %v", err)
	}
	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		match := etcdVersionRegexp.FindStringSubmatch(scanner.Text())
		if len(match) == 2 {
			return match[1], nil
		}
	}
	return "", fmt.Errorf("unable to parse version from etcd output")
}
