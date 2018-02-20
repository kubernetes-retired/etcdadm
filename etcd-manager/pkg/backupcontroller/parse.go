package backupcontroller

import (
	"strings"
	"time"
)

type backupNameInfo struct {
	Timestamp time.Time
	Suffix    string
}

func parseBackupNameInfo(name string) *backupNameInfo {
	info := &backupNameInfo{}

	timeString := name
	z := strings.Index(name, "Z-")
	if z != -1 {
		timeString = name[0 : z+1]
		info.Suffix = name[z+2:]
	}
	t, err := time.Parse(time.RFC3339, timeString)
	if err != nil {
		return nil
	}

	info.Timestamp = t
	return info
}
