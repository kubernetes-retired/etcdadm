package gce

import (
	"fmt"
	"strings"
)

// regionFromZone returns region of the gce zone. Zone names
// are of the form: ${region-name}-${ix}.
// For example, "us-central1-b" has a region of "us-central1".
// So we look for the last '-' and trim to just before that.
func regionFromZone(zone string) (string, error) {
	ix := strings.LastIndex(zone, "-")
	if ix == -1 {
		return "", fmt.Errorf("unexpected zone: %s", zone)
	}
	return zone[:ix], nil
}

// Returns the last component of a URL, i.e. anything after the last slash
// If there is no slash, returns the whole string
func lastComponent(s string) string {
	lastSlash := strings.LastIndex(s, "/")
	if lastSlash != -1 {
		s = s[lastSlash+1:]
	}
	return s
}
