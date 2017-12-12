package dump

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"

	etcd_client "github.com/coreos/etcd/client"
)

type DumpSink interface {
	io.Closer

	Write(node *etcd_client.Node) error
}

type TarDumpSink struct {
	tw  *tar.Writer
	gzw *gzip.Writer
	f   *os.File
}

var _ DumpSink = &TarDumpSink{}

func NewTarDumpSink(p string) (*TarDumpSink, error) {
	f, err := os.Create(p)
	if err != nil {
		return nil, fmt.Errorf("unable to create file %q: %v", p, err)
	}
	defer f.Close()

	gzw := gzip.NewWriter(f)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	return &TarDumpSink{
		tw:  tw,
		gzw: gzw,
		f:   f,
	}, nil
}

func (s *TarDumpSink) Write(node *etcd_client.Node) error {
	hdr := &tar.Header{
		Name: node.Key,
	}
	if node.Dir {
		hdr.Mode = 0755
		hdr.Name += "/"
		hdr.Typeflag = tar.TypeDir
	} else {
		hdr.Mode = 0644
		hdr.Typeflag = tar.TypeReg
		hdr.Size = int64(len(node.Value))
	}

	if hdr.Name != "/" {
		hdr.Name = strings.TrimPrefix(hdr.Name, "/")

		// write the header
		if err := s.tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("error writing tar header: %v", err)
		}

		if !node.Dir && node.Value != "" {
			_, err := s.tw.Write([]byte(node.Value))
			if err != nil {
				return fmt.Errorf("error writing tar data: %v", err)
			}
		}
	}

	return nil
}

func (s *TarDumpSink) Close() error {
	var err error
	errC := s.gzw.Close()
	if errC != nil {
		err = fmt.Errorf("error closing gzip: %v", errC)
	}

	errC = s.tw.Close()
	if errC != nil {
		err = fmt.Errorf("error closing tar: %v", errC)
	}

	errC = s.f.Close()
	if errC != nil {
		err = fmt.Errorf("error closing file: %v", errC)
	}

	return err
}
