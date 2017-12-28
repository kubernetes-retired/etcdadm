package privateapi

type Discovery interface {
	Poll() (map[string]DiscoveryNode, error)
}

type DiscoveryNode struct {
	ID        PeerId             `json:"id"`
	Addresses []DiscoveryAddress `json:"addresses"`
}

type DiscoveryAddress struct {
	Address string `json:"address"`
}
