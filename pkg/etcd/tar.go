package etcd

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/golang/glog"
)

func createTgz(tgzFile string, srcdir string) error {
	tgzOut, err := os.Create(tgzFile)
	if err != nil {
		return fmt.Errorf("error creating file %q: %v", tgzFile, err)
	}
	tgzClosed := false
	defer func() {
		if !tgzClosed {
			tgzOut.Close()
		}
	}()

	gz := gzip.NewWriter(tgzOut)
	w := tar.NewWriter(gz)

	if err := addTreeToTar(w, srcdir, ""); err != nil {
		return err
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("error writing tgz file: %v", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("error writing tgz file: %v", err)
	}
	if err := tgzOut.Close(); err != nil {
		return fmt.Errorf("error writing tgz file: %v", err)
	}

	tgzClosed = true
	return nil
}

type tgzArchive struct {
	File string
}

func (t *tgzArchive) Extract(destDir string) error {
	tgzIn, err := os.Open(t.File)
	if err != nil {
		return fmt.Errorf("error reading file %q: %v", t.File, err)
	}
	defer tgzIn.Close()

	gz, err := gzip.NewReader(tgzIn)
	if err != nil {
		return fmt.Errorf("error opening gzip file %q: %v", t.File, err)
	}
	defer gz.Close()

	r := tar.NewReader(gz)

	for {
		h, err := r.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("error reading tar header (corrupted?): %v", err)
		}

		destFile := filepath.Join(destDir, h.Name)
		if h.FileInfo().IsDir() {
			if err := os.MkdirAll(destFile, 0755); err != nil {
				return fmt.Errorf("error creating directories for %q: %v", destFile, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
			return fmt.Errorf("error creating directories for %q: %v", destFile, err)
		}
		if err := createFile(destFile, r, h.Size); err != nil {
			return err
		}
	}
}

type gzFile struct {
	File string
}

// gunzip will uncompress the file srcFile, writing it to destFile
func (g *gzFile) expand(destFile string) error {
	src, err := os.Open(g.File)
	if err != nil {
		return fmt.Errorf("error opening %q: %v", g.File, err)
	}
	defer src.Close()

	dest, err := os.Create(destFile)
	if err != nil {
		return fmt.Errorf("error opening %q: %v", g.File, err)
	}
	defer dest.Close()

	gz, err := gzip.NewReader(src)
	if err != nil {
		return fmt.Errorf("error reading gzip %q (source corrupted?): %v", g.File, err)
	}

	n, err := io.Copy(dest, gz)
	if err != nil {
		return fmt.Errorf("error expanding file: %v", err)
	}
	glog.V(2).Infof("expanded snapshot file, size=%d bytes", n)

	if err := gz.Close(); err != nil {
		return fmt.Errorf("error completing file expansion: %v", err)
	}

	return nil
}

// addTreeToTar copies the file tree from srcdir, writing it to the tar.Writer with a prefix
func addTreeToTar(w *tar.Writer, srcdir string, prefix string) error {
	files, err := ioutil.ReadDir(srcdir)
	if err != nil {
		return fmt.Errorf("error listing %q: %v", srcdir, err)
	}

	for _, f := range files {
		srcPath := filepath.Join(srcdir, f.Name())

		if f.IsDir() {
			if err := addTreeToTar(w, srcPath, path.Join(prefix, f.Name())); err != nil {
				return err
			}
			continue
		}

		h, err := tar.FileInfoHeader(f, "")
		if err != nil {
			return fmt.Errorf("error building tar file header: %v", err)
		}
		h.Name = path.Join(prefix, h.Name)

		if err := w.WriteHeader(h); err != nil {
			return fmt.Errorf("error writing to tar file: %v", err)
		}

		if err := copyFileToWriter(w, srcPath); err != nil {
			return err
		}
	}

	return nil
}

// copyFile reads the file from srcPath and writes it to w
func copyFileToWriter(w io.Writer, srcPath string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("error reading source file %q: %v", srcPath, err)
	}
	defer f.Close()

	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("error writing to tar file for %q: %v", srcPath, err)
	}

	return nil
}

// createFile creates the named file and copies N bytes from the io.Reader r
func createFile(destFile string, r io.Reader, size int64) error {
	f, err := os.Create(destFile)
	if err != nil {
		return fmt.Errorf("error creating file %q: %v", destFile, err)
	}
	defer f.Close()

	if _, err := io.CopyN(f, r, size); err != nil {
		return fmt.Errorf("error reading file %q: %v", destFile, err)
	}

	return nil
}
