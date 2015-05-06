package main

import (
	"fmt"
	"os"

	"github.com/shashidharatd/gotc/netlink"
)

const tcUsage string = `
tc OBJECT COMMAND
where  OBJECT := { qdisc | filter }
`

const qdiscUsage string = `
qdisc show dev <intf>
qdisc add  dev <intf> root tbf  rate <rate>bit burst <burst> latency <latency
qdisc add  dev <intf> ingress handle <handle>
qdisc del  dev <intf> { root|ingress }
`

func usage(usage string) {
	fmt.Fprintf(os.Stderr, "Usage:%s", usage)
	os.Exit(1)
}

func qdiscAdd(args []string) error {
	fmt.Printf("qdisc add dev called with = %v\n", args)
	return nil
}

func qdiscDel(args []string) error {
	fmt.Printf("qdisc del dev called with = %v\n", args)
	return nil
}

func handleQdisc(args []string) error {
	if len(args) < 3 || args[1] != "dev" {
		usage(qdiscUsage)
	}

	switch args[0] {
	case "show":
		if len(args) != 3 {
			usage(qdiscUsage)
		}
		return netlink.PrintQDisc(args[2])
	case "add":
		return qdiscAdd(args[2:])
	case "del":
		return qdiscDel(args[2:])
	default:
		usage(qdiscUsage)
	}

	return nil
}

func handleFilter(args []string) error {
	fmt.Printf("filter called with args = %v", args)
	return nil
}

func main() {

	args := os.Args
	if len(args) < 2 {
		usage(tcUsage)
	}

	switch args[1] {
	case "qdisc":
		err := handleQdisc(args[2:])
		if err != nil {
			os.Exit(1)
		}
	case "filter":
		err := handleFilter(args[2:])
		if err != nil {
			os.Exit(1)
		}
	default:
		usage(tcUsage)
	}
}
