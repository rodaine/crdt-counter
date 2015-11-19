package main

import (
	"flag"
	"os"
	"os/signal"
	"strings"
)

var (
	selfID    = flag.String("id", "", "id of this counter")
	selfPort  = flag.Int("port", 0, "port of this counter")
	peerHosts = flag.String("peers", "", "comma-separated list of peers (ex: host:port,host:port)")
	self      *Self
)

func main() {
	flag.Parse()
	self = NewSelf(*selfID, *selfPort)

	var peers []string
	if *peerHosts != "" {
		peers = strings.Split(*peerHosts, ",")
	}
	msgs := self.Listen(peers)
	go self.Handle(msgs)

	waitForSignal()
}

func waitForSignal() {
	sigInt := make(chan os.Signal)
	signal.Notify(sigInt, os.Interrupt)
	<-sigInt

	self.Log("SIGINT received, shutting down...")
	os.Exit(0)
}
