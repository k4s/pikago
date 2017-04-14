// Package pull implements the PULL protocol, which is the read side of
// the pipeline pattern.  (PUSH is the reader.)
package pull

import (
	"time"

	"github.com/k4s/pikago"
)

type pull struct {
	sock pikago.ProtocolSocket
	raw  bool
}

func (x *pull) Init(sock pikago.ProtocolSocket) {
	x.sock = sock
	x.sock.SetSendError(pikago.ErrProtoOp)
}

func (x *pull) Shutdown(time.Time) {} // No sender to drain

func (x *pull) receiver(ep pikago.Endpoint) {
	rq := x.sock.RecvChannel()
	cq := x.sock.CloseChannel()
	for {

		m := ep.RecvMsg()
		if m == nil {
			return
		}

		select {
		case rq <- m:
		case <-cq:
			return
		}
	}
}

func (*pull) Number() uint16 {
	return pikago.ProtoPull
}

func (*pull) PeerNumber() uint16 {
	return pikago.ProtoPush
}

func (*pull) Name() string {
	return "pull"
}

func (*pull) PeerName() string {
	return "push"
}

func (x *pull) AddEndpoint(ep pikago.Endpoint) {
	go x.receiver(ep)
}

func (x *pull) RemoveEndpoint(ep pikago.Endpoint) {}

func (*pull) SendHook(msg *pikago.Message) bool {
	return false
}

func (x *pull) SetOption(name string, v interface{}) error {
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

func (x *pull) GetOption(name string) (interface{}, error) {
	switch name {
	case pikago.OptionRaw:
		return x.raw, nil
	default:
		return nil, pikago.ErrBadOption
	}
}

// NewSocket allocates a new Socket using the PULL protocol.
func NewSocket() (pikago.Socket, error) {
	return pikago.MakeSocket(&pull{}), nil
}
