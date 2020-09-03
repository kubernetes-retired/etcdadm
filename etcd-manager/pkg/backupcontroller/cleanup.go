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

package backupcontroller

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog"

	"kope.io/etcd-manager/pkg/backup"
)

// HourlyBackupsRetention controls how long hourly backups are kept.
var HourlyBackupsRetention = 24 * 7 * time.Hour

// DailyBackupsRetention controls how long daily backups are kept.
var DailyBackupsRetention = 24 * 7 * 365 * time.Hour

// ParseHumanDuration parses a go-style duration string, but
// recognizes additional suffixes: d means "day" and is interpreted as
// 24 hours; y means "year" and is interpreted as 365 days.
func ParseHumanDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "y") {
		s = strings.TrimSuffix(s, "y")
		n, err := strconv.Atoi(s)
		if err != nil {
			return time.Duration(0), err
		}
		return time.Duration(n) * 365 * 24 * time.Hour, nil
	}

	if strings.HasSuffix(s, "d") {
		s = strings.TrimSuffix(s, "d")
		n, err := strconv.Atoi(s)
		if err != nil {
			return time.Duration(0), err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}

	return time.ParseDuration(s)

}

func init() {
	if s := os.Getenv("ETCD_MANAGER_HOURLY_BACKUPS_RETENTION"); s != "" {
		v, err := ParseHumanDuration(s)
		if err != nil {
			klog.Fatalf("failed to parse ETCD_MANAGER_HOURLY_BACKUPS_RETENTION=%q", s)
		}
		HourlyBackupsRetention = v
	}

	if s := os.Getenv("ETCD_MANAGER_DAILY_BACKUPS_RETENTION"); s != "" {
		v, err := ParseHumanDuration(s)
		if err != nil {
			klog.Fatalf("failed to parse ETCD_MANAGER_DAILY_BACKUPS_RETENTION=%q", s)
		}
		DailyBackupsRetention = v
	}
}

// BackupCleanup encapsulates the logic around periodically removing old backups
type BackupCleanup struct {
	backupStore backup.Store

	// lastBackupCleanup is the time at which we last performed a backup store cleanup (as leader)
	lastBackupCleanup time.Time

	backupCleanupInterval time.Duration
}

// NewBackupCleanup constructs a BackupCleanup
func NewBackupCleanup(backupStore backup.Store) *BackupCleanup {
	return &BackupCleanup{
		backupStore:           backupStore,
		backupCleanupInterval: time.Hour,
	}
}

// MaybeDoBackupMaintenance removes old backups, if a suitable interval has passed.
// It should be called periodically, after every backup for example.
func (m *BackupCleanup) MaybeDoBackupMaintenance(ctx context.Context) error {
	now := time.Now()

	if now.Sub(m.lastBackupCleanup) < m.backupCleanupInterval {
		return nil
	}

	backupNames, err := m.backupStore.ListBackups()
	if err != nil {
		return fmt.Errorf("error listing backups: %v", err)
	}

	minRetention := time.Hour

	backups := make(map[time.Time]string)
	retain := make(map[string]bool)
	ignore := make(map[string]bool)
	buckets := make(map[time.Time]time.Time)

	for _, backup := range backupNames {
		// Time parsing uses the same layout values as `Format`.
		i := parseBackupNameInfo(backup)
		if i == nil {
			klog.Warningf("ignoring unparseable backup %q", backup)
			ignore[backup] = true
			continue
		}

		t := i.Timestamp
		backups[t] = backup

		age := now.Sub(t)

		if age < minRetention {
			retain[backup] = true
			continue
		}

		if age < HourlyBackupsRetention {
			bucketed := t.Truncate(time.Hour)
			existing := buckets[bucketed]
			if existing.IsZero() || existing.After(t) {
				buckets[bucketed] = t
			}
			continue
		}

		if age < DailyBackupsRetention {
			bucketed := t.Truncate(time.Hour * 24)
			existing := buckets[bucketed]
			if existing.IsZero() || existing.After(t) {
				buckets[bucketed] = t
			}
			continue
		}
	}

	for _, t := range buckets {
		retain[backups[t]] = true
	}

	removedCount := 0
	for _, backup := range backupNames {
		if retain[backup] {
			klog.V(4).Infof("retaining backup %q", backup)
			continue
		}
		if ignore[backup] {
			klog.V(4).Infof("ignoring backup %q", backup)
			continue
		}
		klog.V(4).Infof("removing backup %q", backup)
		if err := m.backupStore.RemoveBackup(backup); err != nil {
			klog.Warningf("failed to remove backup %q: %v", backup, err)
		} else {
			klog.V(2).Infof("removed backup %q", backup)
			removedCount++
		}
	}

	if removedCount != 0 {
		klog.Infof("Removed %d old backups", removedCount)
	}

	m.lastBackupCleanup = now

	return nil
}
