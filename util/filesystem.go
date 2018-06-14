package util

import (
	"os"
)

// FileExists checks whether the file exists
func FileExists(path string) (bool, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, err
	}
	return true, nil
}
