package etcd

import (
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
)

func ToJson(o proto.Message) (string, error) {
	marshaler := jsonpb.Marshaler{
		Indent: "  ",
	}
	return marshaler.MarshalToString(o)
}

func FromJson(s string, o proto.Message) error {
	return jsonpb.UnmarshalString(s, o)
}
