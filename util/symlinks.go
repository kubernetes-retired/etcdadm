package util

import (
	"io/ioutil"
	"log"
	"path/filepath"
	"os"
)


// Create symlinks of all the files inside sourceDir to targetDir
func CreateSymLinks(sourceDir, targetDir string, overwriteSymlinks bool) error {
	files, err := ioutil.ReadDir(sourceDir)
	if err != nil {
		return err
	}
	_, parentDir := filepath.Split(sourceDir)

	for _, f := range files {
		log.Print("Creating symlink for " + f.Name())

		symlinkPath := filepath.Join(targetDir, f.Name())

		if overwriteSymlinks {
			if _, err := os.Lstat(symlinkPath); err == nil {
				os.Remove(symlinkPath)
			}
		}

		err = os.Symlink(filepath.Join(parentDir, f.Name()), symlinkPath)
		if err != nil {
			return err
		}
	}
	return nil
}
