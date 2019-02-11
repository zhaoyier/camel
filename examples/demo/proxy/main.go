package main

import (
	"camel/lizard"
	"fmt"
	"net"

	"github.com/leesper/holmes"
)

func main() {
	defer holmes.Start().Stop()

	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", "0.0.0.0", 10000))
	if err != nil {
		holmes.Fatalln("listen error", err)
	}

	lizard.NewGateway().Start(l)
}
