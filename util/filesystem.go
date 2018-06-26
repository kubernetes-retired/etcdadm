package util

import (
	"os"
)

// FileExists checks whether the file exists
func FileExists(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// RemoveFolder removes the folder and all of its contents
func RemoveFolder(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	return nil
}
