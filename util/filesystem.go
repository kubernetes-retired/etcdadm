package util

import (
	"fmt"
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

// RemoveFolderRecursive removes the folder and all of its contents
func RemoveFolderRecursive(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("[util] Failed recursively removing directory %s : %s", path, err)
	}
	return nil
}

// RemoveFile removes the file/directory specified
func RemoveFile(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("[util] Failed removing path %s : %s", path, err)
	}
	return nil
}

// CopyRecursive copies all contents from srcDir to destDir
func CopyRecursive(srcDir, destDir string) error {
	return Run("", "cp", "-r", srcDir, destDir)
}

// CreateSymLink creates a symbolic link for a input file
func CreateSymLink(srcFile, linkFile string) error {
	RemoveFile(linkFile)
	return os.Symlink(srcFile, linkFile)
}
