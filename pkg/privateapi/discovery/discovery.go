package discovery

type Interface interface {
	Poll() (map[string]Node, error)
}

type Node struct {
	ID        string         `json:"id"`
	Endpoints []NodeEndpoint `json:"endpoints"`
}

type NodeEndpoint struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
}
