package main

import (
	"flag"
	"github.com/warm-metal/kubectl-dev/pkg/session"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
	"net"
)

var addr = flag.String("addr", ":8001", "TCP address to listen on")
var ns = flag.String("app-namespace", "app", "Namespace apps to be installed")

func init() {
	klog.InitFlags(flag.CommandLine)
}

func main() {
	flag.Parse()
	klog.LogToStderr(true)
	defer klog.Flush()
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
