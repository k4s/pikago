// Package push implements the PUSH protocol, which is the write side of
// the pipeline pattern.  (PULL is the reader.)
package push

import (
	"sync"
	"time"

	"github.com/k4s/pikago"
)

type push struct {
	sock pikago.ProtocolSocket
	raw  bool
	w    pikago.Waiter
	eps  map[uint32]*pushEp
	sync.Mutex
}

type pushEp struct {
	ep pikago.Endpoint
	cq chan struct{}
}

func (x *push) Init(sock pikago.ProtocolSocket) {
	x.sock = sock
	x.w.Init()
	x.sock.SetRecvError(pikago.ErrProtoOp)
	x.eps = make(map[uint32]*pushEp)
}

func (x *push) Shutdown(expire time.Time) {
	x.w.WaitAbsTimeout(expire)
}

func (x *push) sender(ep *pushEp) {
	defer x.w.Done()
	sq := x.sock.SendChannel()
	cq := x.sock.CloseChannel()

	for {
		select {
		case <-cq:
			return
		case <-ep.cq:
			return
		case m := <-sq:
			if m == nil {
				sq = x.sock.SendChannel()
				continue
			}
			if ep.ep.SendMsg(m) != nil {
				m.Free()
				return
			}
		}
	}
}

func (*push) Number() uint16 {
	return pikago.ProtoPush
}

func (*push) PeerNumber() uint16 {
	return pikago.ProtoPull
}

func (*push) Name() string {
	return "push"
}

func (*push) PeerName() string {
	return "pull"
}

func (x *push) AddEndpoint(ep pikago.Endpoint) {
	pe := &pushEp{ep: ep, cq: make(chan struct{})}
	x.Lock()
	x.eps[ep.GetID()] = pe
	x.Unlock()
	x.w.Add()
	go x.sender(pe)
	go pikago.NullRecv(ep)
}

func (x *push) RemoveEndpoint(ep pikago.Endpoint) {
	id := ep.GetID()
	x.Lock()
	pe := x.eps[id]
	delete(x.eps, id)
	x.Unlock()
	if pe != nil {
		close(pe.cq)
	}
}

func (x *push) SetOption(name string, v interface{}) error {
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

func (x *push) GetOption(name string) (interface{}, error) {
	switch name {
	case pikago.OptionRaw:
		return x.raw, nil
	default:
		return nil, pikago.ErrBadOption
	}
}

// NewSocket allocates a new Socket using the PUSH protocol.
func NewSocket() (pikago.Socket, error) {
	return pikago.MakeSocket(&push{}), nil
}
