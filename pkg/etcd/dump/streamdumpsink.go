package dump

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"kope.io/etcd-manager/pkg/etcdclient"
)

type StreamDumpSink struct {
	out io.WriteCloser
}

var _ etcdclient.NodeSink = &StreamDumpSink{}

func NewStreamDumpSink(out io.WriteCloser) (*StreamDumpSink, error) {
	return &StreamDumpSink{
		out: out,
	}, nil
}

func (s *StreamDumpSink) Close() error {
	return nil
}

func (s *StreamDumpSink) Put(ctx context.Context, key string, value []byte) error {
	name := key

	if name != "/" {
		name = strings.TrimPrefix(name, "/")

		var b bytes.Buffer
		b.WriteString(fmt.Sprintf("%s => %s\n", name, string(value)))

		if _, err := b.WriteTo(s.out); err != nil {
			return fmt.Errorf("error writing to output: %v", err)
		}
	}

	return nil
}
