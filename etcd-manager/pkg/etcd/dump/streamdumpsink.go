/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
