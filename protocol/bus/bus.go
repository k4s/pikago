// Package bus implements the BUS protocol.  In this protocol, participants
// send a message to each of their peers.
package bus

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/k4s/pikago"
)

type busEp struct {
	ep pikago.Endpoint
	q  chan *pikago.Message
	x  *bus
}

type bus struct {
	sock  pikago.ProtocolSocket
	peers map[uint32]*busEp
	raw   bool
	w     pikago.Waiter
	init  sync.Once

	sync.Mutex
}

// Init implements the Protocol Init method.
func (x *bus) Init(sock pikago.ProtocolSocket) {
	x.sock = sock
	x.peers = make(map[uint32]*busEp)
	x.w.Init()
	x.w.Add()
	go x.sender()
}

func (x *bus) Shutdown(expire time.Time) {

	x.w.WaitAbsTimeout(expire)

	x.Lock()
	peers := x.peers
	x.peers = make(map[uint32]*busEp)
	x.Unlock()

	for id, peer := range peers {
		pikago.DrainChannel(peer.q, expire)
		close(peer.q)
		delete(peers, id)
	}
}

// Bottom sender.
func (pe *busEp) peerSender() {
	for {
		m := <-pe.q
		if m == nil {
			return
		}
		if pe.ep.SendMsg(m) != nil {
			m.Free()
			return
		}
	}
}

func (x *bus) broadcast(m *pikago.Message, sender uint32) {

	x.Lock()
	for id, pe := range x.peers {
		if sender == id {
			continue
		}
		m = m.Dup()

		select {
		case pe.q <- m:
		default:
			// No room on outbound queue, drop it.
			// Note that if we are passing on a linger/shutdown
			// notification and we can't deliver due to queue
			// full, it means we will wind up waiting the full
			// linger time in the lower sender.  Its correct, if
			// suboptimal, behavior.
			m.Free()
		}
	}
	x.Unlock()
}

func (x *bus) sender() {
	cq := x.sock.CloseChannel()
	sq := x.sock.SendChannel()
	defer x.w.Done()
	for {
		var id uint32
		select {
		case <-cq:
			return
		case m := <-sq:
			if m == nil {
				sq = x.sock.SendChannel()
				continue
			}
			// If a header was present, it means this message is
			// being rebroadcast.  It should be a pipe ID.
			if len(m.Header) >= 4 {
				id = binary.BigEndian.Uint32(m.Header)
				m.Header = m.Header[4:]
			}
			x.broadcast(m, id)
			m.Free()
		}
	}
}

func (pe *busEp) receiver() {

	rq := pe.x.sock.RecvChannel()
	cq := pe.x.sock.CloseChannel()

	for {
		m := pe.ep.RecvMsg()
		if m == nil {
			return
		}
		v := pe.ep.GetID()
		m.Header = append(m.Header,
			byte(v>>24), byte(v>>16), byte(v>>8), byte(v))

		select {
		case rq <- m:
		case <-cq:
			m.Free()
			return
		default:
			// No room, so we just drop it.
			m.Free()
		}
	}
}

func (x *bus) AddEndpoint(ep pikago.Endpoint) {
	// Set our broadcast depth to match upper depth -- this should
	// help avoid dropping when bursting, if we burst before we
	// context switch.
	depth := 16
	if i, err := x.sock.GetOption(pikago.OptionWriteQLen); err == nil {
		depth = i.(int)
	}
	pe := &busEp{ep: ep, x: x, q: make(chan *pikago.Message, depth)}
	x.Lock()
	x.peers[ep.GetID()] = pe
	x.Unlock()
	go pe.peerSender()
	go pe.receiver()
}

func (x *bus) RemoveEndpoint(ep pikago.Endpoint) {
	x.Lock()
	if peer := x.peers[ep.GetID()]; peer != nil {
		close(peer.q)
		delete(x.peers, ep.GetID())
	}
	x.Unlock()
}

func (*bus) Number() uint16 {
	return pikago.ProtoBus
}

func (*bus) Name() string {
	return "bus"
}

func (*bus) PeerNumber() uint16 {
	return pikago.ProtoBus
}

func (*bus) PeerName() string {
	return "bus"
}

func (x *bus) RecvHook(m *pikago.Message) bool {
	if !x.raw && len(m.Header) >= 4 {
		m.Header = m.Header[4:]
	}
	return true
}

func (x *bus) SetOption(name string, v interface{}) error {
	var ok bool
	switch name {
	case pikago.OptionRaw:
		if x.raw, ok = v.(bool); !ok {
			return pikago.ErrBadValue
		}
		return nil
	default:
		return pikago.ErrBadOption
	}
}

func (x *bus) GetOption(name string) (interface{}, error) {
	switch name {
	case pikago.OptionRaw:
		return x.raw, nil
	default:
		return nil, pikago.ErrBadOption
	}
}

// NewSocket allocates a new Socket using the BUS protocol.
func NewSocket() (pikago.Socket, error) {
	return pikago.MakeSocket(&bus{}), nil
}
