package discovery

type Interface interface {
	Poll() (map[string]Node, error)
}

type Node struct {
	ID        string        `json:"id"`
	Addresses []NodeAddress `json:"addresses"`
}

type NodeAddress struct {
	Address string `json:"address"`
}
