// Package rep implements the REP protocol, which is the response side of
// the request/response pattern.  (REQ is the request.)
package rep

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/k4s/pikago"
)

type repEp struct {
	q    chan *pikago.Message
	ep   pikago.Endpoint
	sock pikago.ProtocolSocket
	w    pikago.Waiter
	r    *rep
}

type rep struct {
	sock         pikago.ProtocolSocket
	eps          map[uint32]*repEp
	backtracebuf []byte
	backtrace    []byte
	backtraceL   sync.Mutex
	raw          bool
	ttl          int
	w            pikago.Waiter

	sync.Mutex
}

func (r *rep) Init(sock pikago.ProtocolSocket) {
	r.sock = sock
	r.eps = make(map[uint32]*repEp)
	r.backtracebuf = make([]byte, 64)
	r.ttl = 8 // default specified in the RFC
	r.w.Init()
	r.sock.SetSendError(pikago.ErrProtoState)
	r.w.Add()
	go r.sender()
}

func (r *rep) Shutdown(expire time.Time) {

	r.w.WaitAbsTimeout(expire)

	r.Lock()
	peers := r.eps
	r.eps = make(map[uint32]*repEp)
	r.Unlock()

	for id, peer := range peers {
		delete(peers, id)
		pikago.DrainChannel(peer.q, expire)
		close(peer.q)
	}
}

func (pe *repEp) sender() {
	for {
		m := <-pe.q
		if m == nil {
			break
		}

		if pe.ep.SendMsg(m) != nil {
			m.Free()
			break
		}
	}
}

func (r *rep) receiver(ep pikago.Endpoint) {

	rq := r.sock.RecvChannel()
	cq := r.sock.CloseChannel()

	for {

		m := ep.RecvMsg()
		if m == nil {
			return
		}

		v := ep.GetID()
		m.Header = append(m.Header,
			byte(v>>24), byte(v>>16), byte(v>>8), byte(v))

		hops := 0
		// Move backtrace from body to header.
		for {
			if hops >= r.ttl {
				m.Free() // ErrTooManyHops
				return
			}
			hops++
			if len(m.Body) < 4 {
				m.Free() // ErrGarbled
				return
			}
			m.Header = append(m.Header, m.Body[:4]...)
			m.Body = m.Body[4:]
			// Check for high order bit set (0x80000000, big endian)
			if m.Header[len(m.Header)-4]&0x80 != 0 {
				break
			}
		}

		select {
		case rq <- m:
		case <-cq:
			m.Free()
			return
		}
	}
}

func (r *rep) sender() {
	defer r.w.Done()
	cq := r.sock.CloseChannel()
	sq := r.sock.SendChannel()

	for {
		var m *pikago.Message

		select {
		case m = <-sq:
			if m == nil {
				sq = r.sock.SendChannel()
				continue
			}
		case <-cq:
			return
		}

		// Lop off the 32-bit peer/pipe ID.  If absent, drop.
		if len(m.Header) < 4 {
			m.Free()
			continue
		}
		id := binary.BigEndian.Uint32(m.Header)
		m.Header = m.Header[4:]
		r.Lock()
		pe := r.eps[id]
		r.Unlock()
		if pe == nil {
			m.Free()
			continue
		}

		select {
		case pe.q <- m:
		default:
			// If our queue is full, we have no choice but to
			// throw it on the floor.  This shoudn't happen,
			// since each partner should be running synchronously.
			// Devices are a different situation, and this could
			// lead to lossy behavior there.  Initiators will
			// resend if this happens.  Devices need to have deep
			// enough queues and be fast enough to avoid this.
			m.Free()
		}
	}
}

func (*rep) Number() uint16 {
	return pikago.ProtoRep
}

func (*rep) PeerNumber() uint16 {
	return pikago.ProtoReq
}

func (*rep) Name() string {
	return "rep"
}

func (*rep) PeerName() string {
	return "req"
}

func (r *rep) AddEndpoint(ep pikago.Endpoint) {
	pe := &repEp{ep: ep, r: r, q: make(chan *pikago.Message, 2)}
	pe.w.Init()
	r.Lock()
	r.eps[ep.GetID()] = pe
	r.Unlock()
	go r.receiver(ep)
	go pe.sender()
}

func (r *rep) RemoveEndpoint(ep pikago.Endpoint) {
	id := ep.GetID()

	r.Lock()
	pe := r.eps[id]
	delete(r.eps, id)
	r.Unlock()

	if pe != nil {
		close(pe.q)
	}
}

// We save the backtrace from this message.  This means that if the app calls
// Recv before calling Send, the saved backtrace will be lost.  This is how
// the application discards / cancels a request to which it declines to reply.
// This is only done in cooked mode.
func (r *rep) RecvHook(m *pikago.Message) bool {
	if r.raw {
		return true
	}
	r.sock.SetSendError(nil)
	r.backtraceL.Lock()
	r.backtrace = append(r.backtracebuf[0:0], m.Header...)
	r.backtraceL.Unlock()
	m.Header = nil
	return true
}

func (r *rep) SendHook(m *pikago.Message) bool {
	// Store our saved backtrace.  Note that if none was previously stored,
	// there is no one to reply to, and we drop the message.  We only
	// do this in cooked mode.
	if r.raw {
		return true
	}
	r.sock.SetSendError(pikago.ErrProtoState)
	r.backtraceL.Lock()
	m.Header = append(m.Header[0:0], r.backtrace...)
	r.backtrace = nil
	r.backtraceL.Unlock()
	if m.Header == nil {
		return false
	}
	return true
}

func (r *rep) SetOption(name string, v interface{}) error {
	var ok bool
	switch name {
	case pikago.OptionRaw:
		if r.raw, ok = v.(bool); !ok {
			return pikago.ErrBadValue
		}
		if r.raw {
			r.sock.SetSendError(nil)
		} else {
			r.sock.SetSendError(pikago.ErrProtoState)
		}
		return nil
	case pikago.OptionTTL:
		if ttl, ok := v.(int); !ok {
			return pikago.ErrBadValue
		} else if ttl < 1 || ttl > 255 {
			return pikago.ErrBadValue
		} else {
			r.ttl = ttl
		}
		return nil
	default:
		return pikago.ErrBadOption
	}
}

func (r *rep) GetOption(name string) (interface{}, error) {
	switch name {
	case pikago.OptionRaw:
		return r.raw, nil
	case pikago.OptionTTL:
		return r.ttl, nil
	default:
		return nil, pikago.ErrBadOption
	}
}

// NewSocket allocates a new Socket using the REP protocol.
func NewSocket() (pikago.Socket, error) {
	return pikago.MakeSocket(&rep{}), nil
}
