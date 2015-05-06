package main

import (
	"fmt"
	"os"

	"github.com/docker/libcontainer/netlink"
)

func main() {

	args := os.Args

	name1 := "zeth01"
	name2 := "zeth02"

	if len(args) > 1 {

		netlink.NetworkLinkDel(name1)
		netlink.NetworkLinkDel(name2)

	} else {
		if err := netlink.NetworkCreateVethPair(name1, name2, 0); err != nil {
			fmt.Printf("Could not create veth pair %s %s: %s", name1, name2, err)
			return
		}
	}

}
