package discovery

type Interface interface {
	Poll() (map[string]Node, error)
}

type Node struct {
	ID        string         `json:"id"`
	Endpoints []NodeEndpoint `json:"endpoints"`
}

type NodeEndpoint struct {
	Endpoint string `json:"endpoint"`
}
