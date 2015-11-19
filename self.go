package main

import (
	"crypto/rand"
	"fmt"
	"net"
	"strconv"
	"sync"
)

type Self struct {
	Peer
	sync.Once

	Port    int
	Peers   map[string]*Peer
	Sniffed map[string]bool

	Partitioned bool

	plock sync.RWMutex
	slock sync.RWMutex
}

func NewSelf(id string, port int) *Self {
	if id == "" {
		b := make([]byte, 8)
		rand.Read(b)
		id = fmt.Sprintf("%x", b)
	}

	return &Self{
		Peer:    Peer{ID: id},
		Port:    port,
		Peers:   make(map[string]*Peer),
		Sniffed: make(map[string]bool),
	}
}

func (s *Self) Listen(peers []string) <-chan Message {
	addr, err := net.ResolveUDPAddr("udp", "[::]:"+strconv.Itoa(s.Port))
	self.CheckError(err)

	conn, err := net.ListenUDP("udp", addr)
	s.CheckError(err)
	s.addr = conn.LocalAddr().(*net.UDPAddr)

	out := make(chan Message)

	go func() {
		buf := make([]byte, 1024)
		for {
			s.Do(func() {
				go self.SniffPeers(peers)
				s.Log("listening for messages on :%d", s.addr.Port)
			})

			n, addr, err := conn.ReadFromUDP(buf)
			//<-simulateLatency(5 * time.Second)
			if err != nil {
				s.Log("error: %v", err)
			} else {
				msg := ParseMessage(string(buf[:n]))
				msg.Sender = addr
				msg.Conn = conn
				out <- msg
			}
		}
	}()

	return out
}

func (s *Self) Send(p *Peer, m Message) {
	if s.Partitioned {
		return
	}

	if err := p.Dial(s); err != nil {
		s.Log("Cannot dial peer %s - %v", p.ID, err)
		return
	}

	if _, err := fmt.Fprint(p.conn, m); err != nil {
		s.Log("Error sending to peer %s - %v", p.ID, err)
	}
}

func (s *Self) Respond(conn *net.UDPConn, addr *net.UDPAddr, m Message) {
	var err error
	if conn == nil {
		addrL, _ := net.ResolveUDPAddr("udp", "[::]:0")

		conn, err = net.DialUDP("udp", addrL, addr)
		if err != nil {
			s.Log("Cannot dial address %v - %v", addr, err)
			return
		}

		_, err = fmt.Fprint(conn, m)
	} else {
		_, err = conn.WriteToUDP([]byte(m.String()), addr)
	}

	if err != nil {
		s.Log("Error sending to address %v - $v", addr, err)
		return
	}
}

func (s *Self) Broadcast(m Message) {
	s.plock.RLock()
	defer s.plock.RUnlock()

	for _, p := range s.Peers {
		s.Send(p, m)
	}
}

func (s *Self) Ping(p *Peer) {
	s.Send(p, Message{
		Type: PingMessage,
		Body: []string{s.ID, s.addr.String(), strconv.Itoa(s.Count())},
	})
}

func (s *Self) SharePeers(p *Peer) {
	s.plock.RLock()
	defer s.plock.RUnlock()

	if len(s.Peers) == 0 {
		return
	}

	hosts := make([]string, len(s.Peers))
	i := 0
	for _, p := range s.Peers {
		hosts[i] = p.addr.String()
		i++
	}

	s.Send(p, Message{
		Type: PeersMessage,
		Body: hosts,
	})
}

func (s *Self) Increment() {
	s.Lock()
	defer s.Unlock()

	s.count++
	s.Log("count updated (%d). sharing with peers...", s.count)
}

func (s *Self) SniffPeers(peerAddr []string) {
	if s.Partitioned {
		return
	}

	if len(peerAddr) == 0 {
		return
	}

	s.slock.RLock()
	defer s.slock.RUnlock()

	for _, peer := range peerAddr {
		addr, err := net.ResolveUDPAddr("udp", peer)
		if err != nil {
			s.Log("invalid peer address `%s`: %v", peer, err)
			continue
		}
		if addr.String() == s.addr.String() {
			continue
		}
		if _, found := s.Sniffed[addr.String()]; found {
			continue
		}
		s.Log("sniffing for peer: %v", peer)
		s.Respond(nil, addr, Message{
			Type: SniffMessage,
			Body: []string{s.ID, s.addr.String(), strconv.Itoa(s.Count())},
		})
		s.Sniffed[addr.String()] = true
	}
}

func (s *Self) MergePeer(p *Peer) {
	if s.Partitioned {
		return
	}

	if p.ID == s.ID {
		return
	}

	s.plock.Lock()
	defer s.plock.Unlock()

	if old, found := s.Peers[p.ID]; found {
		old.Merge(p.Count())
		old.ChangeAddr(p.addr)
		s.Log("Updated peer %v", p)
	} else {
		s.Log("Added peer %v", p)
		s.Peers[p.ID] = p
	}
}

func (s *Self) Total() int {
	s.plock.RLock()
	defer s.plock.RUnlock()

	ct := s.Count()
	for _, p := range s.Peers {
		ct += p.Count()
	}

	return ct
}

func (s *Self) Unpartition() {
	s.slock.Lock()

	peers := make([]string, 0, len(s.Sniffed))
	for _, peer := range s.Peers {
		peers = append(peers, peer.addr.String())
	}

	s.Log("%+v", peers)

	s.Sniffed = map[string]bool{}
	s.slock.Unlock()
	s.SniffPeers(peers)
}

func (s *Self) Handle(msgs <-chan Message) {
	for msg := range msgs {
		switch msg.Type {
		case SniffMessage:
			p := msg.ResolvePeer()
			s.SharePeers(p)
			s.MergePeer(p)
			s.Ping(p)
		case PeersMessage:
			s.SniffPeers(msg.Body)
		case PingMessage:
			s.MergePeer(msg.ResolvePeer())
		case IncrementMessage:
			s.Increment()
			s.Broadcast(Message{
				Type: PingMessage,
				Body: []string{s.ID, s.addr.String(), strconv.Itoa(s.Count())},
			})
		case GetMessage:
			s.Respond(msg.Conn, msg.Sender, Message{Type: strconv.Itoa(s.Total()) + "\n"})
		case PartitionMessage:
			s.Partitioned = true
			s.Log("partitioned")
		case UnpartitionMessage:
			s.Partitioned = false
			s.Unpartition()
			s.Log("unpartitioned")
		default:
			s.Log("unknown message type: %s", msg.Type)
		}
	}
}
