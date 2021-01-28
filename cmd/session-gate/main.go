package main

import (
	"flag"
	"github.com/warm-metal/kubectl-dev/pkg/session"
	"google.golang.org/grpc"
	"net"
)

var addr = flag.String("addr", ":8001", "TCP address to listen on")
var ns = flag.String("app-namespace", "app", "Namespace apps to be installed")

func main() {
	flag.Parse()
	s := grpc.NewServer()
	session.PrepareGate(s, &session.GateOptions{
		Namespace: *ns,
	})
	l, err := net.Listen("tcp", *addr)
	if err != nil {
		panic(err)
	}

	if err = s.Serve(l); err != nil {
		panic(err)
	}
}
