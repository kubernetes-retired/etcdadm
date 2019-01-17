package locking

import (
	"encoding/json"
	"fmt"
)

type LockInfo struct {
	Holder    string `json:"owner"`
	Timestamp int64  `json:"timestamp"`
}

// String implements Stringer
func (l *LockInfo) String() string {
	return fmt.Sprintf("owner=%s, timestamp=%d", l.Holder, l.Timestamp)
}

func (l *LockInfo) ToJSON() ([]byte, error) {
	return json.Marshal(l)
}
