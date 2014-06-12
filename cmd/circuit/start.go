// Copyright 2013 The Go Circuit Project
// Use of this source code is governed by the license for
// The Go Circuit Project, found in the LICENSE file.
//
// Authors:
//   2013 Petar Maymounkov <p@gocircuit.org>

// This package provides the executable program for the resource-sharing circuit app
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path"

	"github.com/gocircuit/circuit/kit/assemble"
	"github.com/gocircuit/circuit/kinfolk"
	"github.com/gocircuit/circuit/kinfolk/locus"
	"github.com/gocircuit/circuit/use/circuit"
	"github.com/gocircuit/circuit/use/n"

	"github.com/gocircuit/circuit/github.com/codegangsta/cli"
)

func server(c *cli.Context) {
	println("CIRCUIT 2014 gocircuit.org")
	var err error

	// parse arguments
	var tcpaddr = parseAddr(c) // server bind address
	var join n.Addr // join address of another circuit server
	if c.IsSet("join") {
		if join, err = n.ParseAddr(c.String("join")); err != nil {
			log.Fatalf("join address does not parse (%s)", err)
		}
	}
	// assemble system udp multicast address
	var multicast *net.UDPAddr
	if c.IsSet("discover") {
		if multicast, err = net.ResolveUDPAddr("udp", c.String("discover")); err != nil {
			log.Fatalf("udp multicast address for discovery and assembly does not parse (%s)", err)
		}
	}
	// server instance working directory
	var varDir string
	if !c.IsSet("var") {
		varDir = path.Join(os.TempDir(), fmt.Sprintf("%s-%%W-P%04d", n.Scheme, os.Getpid()))
	} else {
		varDir = c.String("var")
	}

	// start circuit runtime
	addr := load(tcpaddr, varDir, readkey(c))

	// kinfolk + locus
	kin, xkin, rip := kinfolk.NewKin()
	xlocus := locus.NewLocus(kin, rip)

	// joining
	switch {
	case join != nil:
		kin.ReJoin(join)
	case multicast != nil:
		go assemble.NewAssembler(addr, multicast).AssembleServer(
			func(joinAddr n.Addr) {
				kin.ReJoin(joinAddr)
			},
		)
	default:
		log.Println("Singleton server.")
	}

	circuit.Listen(kinfolk.ServiceName, xkin)
	circuit.Listen(LocusName, xlocus)

	<-(chan int)(nil)
}

func parseAddr(c *cli.Context) *net.TCPAddr {
	switch {
	case c.IsSet("addr"):
		addr, err := net.ResolveTCPAddr("tcp", c.String("addr"))
		if err != nil {
			log.Fatalf("resolve %s (%s)\n", addr, err)
		}
		if len(addr.IP) == 0 {
			addr.IP = net.IPv4zero
		}
		return addr

	case c.IsSet("if"):
		ifc, err := net.InterfaceByName(c.String("if"))
		if err != nil {
			log.Fatalf("interface %s not found (%v)", c.String("if"), err)
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			log.Fatalf("interface address cannot be retrieved (%v)", err)
		}
		if len(addrs) == 0 {
			log.Fatalf("no addresses associated with this interface")
		}
		for _, a := range addrs { // pick the IPv4 one
			ipn := a.(*net.IPNet)
			if ipn.IP.To4() == nil {
				continue
			}
			return &net.TCPAddr{IP: ipn.IP}
		}
		log.Fatal("specified interface has no IPv4 addresses")
	default:
		log.Fatal("either an -addr or an -if option is required to start a server")
	}
	panic(0)
}

const LocusName = "locus"

func dontPanic(call func(), ifPanic string) {
	defer func() {
		if r := recover(); r != nil {
			log.Fatalf("%s (%s)", ifPanic, r)
		}
	}()
	call()
}
