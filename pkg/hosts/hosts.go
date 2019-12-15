/*
Copyright 2017 The Kubernetes Authors.

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

package hosts

import (
	"bytes"
	"fmt"
	"io/ioutil"
	math_rand "math/rand"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"k8s.io/klog"
	"kope.io/etcd-manager/pkg/dns"
)

const GUARD_BEGIN_TEMPLATE = "# Begin host entries managed by __key__ - do not edit"
const GUARD_END_TEMPLATE = "# End host entries managed by __key__"

type Provider struct {
	Key string

	mutex     sync.Mutex
	primary   map[string][]string
	fallbacks map[string][]net.IP
}

var _ dns.Provider = &Provider{}

func (h *Provider) AddFallbacks(dnsFallbacks map[string][]net.IP) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.fallbacks = dnsFallbacks
	return h.update()
}

func (h *Provider) UpdateHosts(addressToHosts map[string][]string) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.primary = addressToHosts
	return h.update()
}

func (h *Provider) update() error {
	addrToHosts := make(map[string][]string)
	hostExists := make(map[string]bool)
	for addr, hosts := range h.primary {
		addrToHosts[addr] = hosts
		for _, h := range hosts {
			hostExists[h] = true
		}
	}

	for host, ips := range h.fallbacks {
		if hostExists[host] {
			continue
		}

		for _, ip := range ips {
			addr := ip.String()
			addrToHosts[addr] = append(addrToHosts[addr], host)
		}
	}

	klog.Infof("hosts update: primary=%v, fallbacks=%v, final=%v", h.primary, h.fallbacks, addrToHosts)
	return updateHostsFileWithRecords("/etc/hosts", h.Key, addrToHosts)
}

func updateHostsFileWithRecords(p string, key string, addrToHosts map[string][]string) error {
	stat, err := os.Stat(p)
	if err != nil {
		return fmt.Errorf("error getting file status of %q: %v", p, err)
	}

	data, err := ioutil.ReadFile(p)
	if err != nil {
		return fmt.Errorf("error reading file %q: %v", p, err)
	}

	guardBegin := strings.Replace(GUARD_BEGIN_TEMPLATE, "__key__", key, -1)
	guardEnd := strings.Replace(GUARD_END_TEMPLATE, "__key__", key, -1)

	var out []string
	inGuardBlock := false
	skipBlank := false
	for _, line := range strings.Split(string(data), "\n") {
		k := strings.TrimSpace(line)

		if k == "" && skipBlank {
			skipBlank = false
			continue
		}
		skipBlank = false

		if k == guardBegin {
			if inGuardBlock {
				klog.Warningf("/etc/hosts guard-block begin seen while in guard block; will ignore")
			}
			inGuardBlock = true
		}

		if !inGuardBlock {
			out = append(out, line)
		}

		if k == guardEnd {
			if !inGuardBlock {
				klog.Warningf("/etc/hosts guard-block end seen before guard-block start; will ignore end")
				// Don't output the line
				out = out[:len(out)-1]
			}

			inGuardBlock = false

			// Avoid problem where we build up lots of whitespace - clean up our blank line
			skipBlank = true
		}
	}

	// Ensure a single blank line
	for {
		if len(out) == 0 {
			break
		}

		if out[len(out)-1] != "" {
			break
		}

		out = out[:len(out)-1]
	}
	out = append(out, "")

	var lines []string
	for addr, hosts := range addrToHosts {
		sort.Strings(hosts)
		var line strings.Builder
		line.WriteString(addr)
		line.WriteString("\t")
		// manual strings.Join(hosts, " ") that skips duplicates
		for i, host := range hosts {
			// skip duplicates
			if i != 0 && hosts[i-1] == host {
				continue
			}
			if i != 0 {
				line.WriteString(" ")
			}
			line.WriteString(host)
		}
		lines = append(lines, line.String())
	}
	sort.Strings(lines)

	out = append(out, guardBegin)
	out = append(out, lines...)
	out = append(out, guardEnd)
	out = append(out, "")

	updated := []byte(strings.Join(out, "\n"))
	if bytes.Equal(updated, data) {
		klog.V(2).Infof("skipping update of unchanged /etc/hosts")
		return nil
	}

	// Note that because we are bind mounting /etc/hosts, we can't do a normal atomic file write
	// (where we write a temp file and rename it)
	// TODO: We should just hold the file open while we read & write it
	containerized := true
	if containerized {
		if err := pseudoAtomicWrite(p, updated, stat.Mode()); err != nil {
			return fmt.Errorf("error writing file %q: %v", p, err)
		}
	} else {
		if err := atomicWriteFile(p, updated, stat.Mode()); err != nil {
			return fmt.Errorf("error writing file %q: %v", p, err)
		}
	}

	return nil
}

// Because we are bind-mounting /etc/hosts, we can't do a normal
// atomic file write (where we write a temp file and rename it);
// instead we write the file, pause, re-read and see if anyone else
// wrote in the meantime; if so we rewrite again.  By pausing for a
// random amount of time, eventually we'll win the write race and
// exit.  This doesn't guarantee fairness, but it should mean that the
// end-result is not malformed (i.e. partial writes).
func pseudoAtomicWrite(p string, b []byte, mode os.FileMode) error {
	attempt := 0
	for {
		attempt++
		if attempt > 10 {
			return fmt.Errorf("failed to consistently write file %q - too many retries", p)
		}

		if err := ioutil.WriteFile(p, b, mode); err != nil {
			klog.Warningf("error writing file %q: %v", p, err)
			continue
		}

		n := 1 + math_rand.Intn(20)
		time.Sleep(time.Duration(n) * time.Millisecond)

		contents, err := ioutil.ReadFile(p)
		if err != nil {
			klog.Warningf("error re-reading file %q: %v", p, err)
			continue
		}

		if bytes.Equal(contents, b) {
			return nil
		}

		klog.Warningf("detected concurrent write to file %q, will retry", p)
	}
}

func atomicWriteFile(filename string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(filename)

	tempFile, err := ioutil.TempFile(dir, ".tmp"+filepath.Base(filename))
	if err != nil {
		return fmt.Errorf("error creating temp file in %q: %v", dir, err)
	}

	mustClose := true
	mustRemove := true

	defer func() {
		if mustClose {
			if err := tempFile.Close(); err != nil {
				klog.Warningf("error closing temp file: %v", err)
			}
		}

		if mustRemove {
			if err := os.Remove(tempFile.Name()); err != nil {
				klog.Warningf("error removing temp file %q: %v", tempFile.Name(), err)
			}
		}
	}()

	if _, err := tempFile.Write(data); err != nil {
		return fmt.Errorf("error writing temp file: %v", err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("error closing temp file: %v", err)
	}

	mustClose = false

	if err := os.Chmod(tempFile.Name(), perm); err != nil {
		return fmt.Errorf("error changing mode of temp file: %v", err)
	}

	if err := os.Rename(tempFile.Name(), filename); err != nil {
		return fmt.Errorf("error moving temp file %q to %q: %v", tempFile.Name(), filename, err)
	}

	mustRemove = false
	return nil
}
