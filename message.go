package main

import (
	"net"
	"strconv"
	"strings"
)

const (
	PingMessage        = "ping"
	SniffMessage       = "sniff"
	PeersMessage       = "peers"
	IncrementMessage   = "inc"
	GetMessage         = "get"
	PartitionMessage   = "part"
	UnpartitionMessage = "unpart"
)

type Message struct {
	Type   string
	Body   []string
	Sender *net.UDPAddr
	Conn   *net.UDPConn
}

func ParseMessage(s string) Message {
	parts := strings.Split(s, " ")
	msg := Message{Type: parts[0]}

	if len(parts) > 1 {
		msg.Body = parts[1:]
	}

	return msg
}

func (m Message) ResolvePeer() *Peer {
	id := m.Body[0]
	addr, _ := net.ResolveUDPAddr("udp", m.Body[1])
	ct, _ := strconv.Atoi(m.Body[2])
	return NewPeer(id, addr, ct)
}

func (m Message) String() string {
	return strings.TrimSpace(m.Type + " " + strings.Join(m.Body, " "))
}
