package main

import (
	"fmt"
	"log"

	netutil "k8s.io/apimachinery/pkg/util/net"
)

func main() {
	ip, err := netutil.ChooseHostInterface()
	if err != nil {
		log.Fatalf("failed to choose host interface: %s", err)
	}
	fmt.Println(ip)
}
