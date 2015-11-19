package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"sync"
)

type Peer struct {
	sync.RWMutex

	ID    string
	count int

	addr *net.UDPAddr
	conn *net.UDPConn
}

func NewPeer(id string, addr *net.UDPAddr, ct int) *Peer {
	return &Peer{
		ID:    id,
		addr:  addr,
		count: ct,
	}
}

func (p *Peer) Count() int {
	p.RLock()
	defer p.RUnlock()

	return p.count
}

func (p *Peer) Merge(new int) {
	p.Lock()
	defer p.Unlock()

	if new > p.count {
		p.count = new
	}
}

func (p *Peer) Log(format string, v ...interface{}) {
	var msg string
	if len(v) > 0 {
		msg = fmt.Sprintf(format, v...)
	} else {
		msg = format
	}
	log.Printf("[%s] %s", p.ID, msg)
}

func (p *Peer) CheckError(err error) {
	if err != nil {
		p.Log("error: %v", err)
		os.Exit(1)
	}
}

func (p *Peer) Dial(s *Self) (err error) {
	if p.conn != nil {
		return nil
	}

	addrL, _ := net.ResolveUDPAddr("udp", ":0")
	p.conn, err = net.DialUDP("udp", addrL, p.addr)
	return
}

func (p *Peer) ChangeAddr(addr *net.UDPAddr) {
	if addr.String() == p.addr.String() {
		return
	}

	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
	}

	p.addr = addr
}

func (p *Peer) String() string {
	return fmt.Sprintf("%s[:%d](%d)", p.ID, p.addr.Port, p.Count())
}
