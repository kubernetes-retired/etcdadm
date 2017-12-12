package locking

import "encoding/json"

type LockInfo struct {
	Holder    string `json:"owner"`
	Timestamp int64  `json:"timestamp"`
}

func (l *LockInfo) ToJSON() ([]byte, error) {
	return json.Marshal(l)
}
