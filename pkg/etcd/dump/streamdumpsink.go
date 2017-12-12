package dump

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	etcd_client "github.com/coreos/etcd/client"
)

type StreamDumpSink struct {
	out io.WriteCloser
}

var _ DumpSink = &StreamDumpSink{}

func NewStreamDumpSink(out io.WriteCloser) (*StreamDumpSink, error) {
	return &StreamDumpSink{
		out: out,
	}, nil
}

func (s *StreamDumpSink) Close() error {
	return nil
}

func (s *StreamDumpSink) Write(node *etcd_client.Node) error {
	name := node.Key

	if node.Dir {
		name += "/"
	}

	if name != "/" {
		name = strings.TrimPrefix(name, "/")

		var b bytes.Buffer
		if !node.Dir {
			b.WriteString(fmt.Sprintf("%s => %s\n", name, node.Value))
		} else {
			b.WriteString(fmt.Sprintf("%s\n", name))
		}

		if _, err := b.WriteTo(s.out); err != nil {
			return fmt.Errorf("error writing to output: %v", err)
		}
	}

	return nil
}
