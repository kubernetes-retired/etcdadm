package dns

import "net"

type Provider interface {
	// AddFallbacks allows specification of some discovery-inferred names
	// These names are used only if they are not otherwise defined.
	// This allows moving from legacy configurations to etcd-manager
	AddFallbacks(dnsFallbacks map[string][]net.IP) error

	// UpdateHosts is used for verified names; these take precedence over the fallbacks
	UpdateHosts(addToHost map[string][]string) error
}
