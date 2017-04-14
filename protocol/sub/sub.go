package sub

import (
	"bytes"
	"sync"
	"time"

	"github.com/k4s/pikago"
)

type sub struct {
	sock pikago.ProtocolSocket
	subs [][]byte
	raw  bool
	sync.Mutex
}

func (s *sub) Init(sock pikago.ProtocolSocket) {
	s.sock = sock
	s.subs = [][]byte{}
	s.sock.SetSendError(pikago.ErrProtoOp)
}

func (*sub) Shutdown(time.Time) {} // No sender to drain.

func (s *sub) receiver(ep pikago.Endpoint) {

	rq := s.sock.RecvChannel()
	cq := s.sock.CloseChannel()

	for {
		var matched = false

		m := ep.RecvMsg()
		if m == nil {
			return
		}

		s.Lock()
		for _, sub := range s.subs {
			if bytes.HasPrefix(m.Body, sub) {
				// Matched, send it up.  Best effort.
				matched = true
				break
			}
		}
		s.Unlock()

		if !matched {
			m.Free()
			continue
		}

		select {
		case rq <- m:
		case <-cq:
			m.Free()
			return
		default: // no room, drop it
			m.Free()
		}
	}
}

func (*sub) Number() uint16 {
	return pikago.ProtoSub
}

func (*sub) PeerNumber() uint16 {
	return pikago.ProtoPub
}

func (*sub) Name() string {
	return "sub"
}

func (*sub) PeerName() string {
	return "pub"
}

func (s *sub) AddEndpoint(ep pikago.Endpoint) {
	go s.receiver(ep)
}

func (*sub) RemoveEndpoint(pikago.Endpoint) {}

func (s *sub) SetOption(name string, value interface{}) error {
	s.Lock()
	defer s.Unlock()

	var vb []byte
	var ok bool

	// Check names first, because type check below is only valid for
	// subscription options.
	switch name {
	case pikago.OptionRaw:
		if s.raw, ok = value.(bool); !ok {
			return pikago.ErrBadValue
		}
		return nil
	case pikago.OptionSubscribe:
	case pikago.OptionUnsubscribe:
	default:
		return pikago.ErrBadOption
	}

	switch v := value.(type) {
	case []byte:
		vb = v
	case string:
		vb = []byte(v)
	default:
		return pikago.ErrBadValue
	}
	switch name {
	case pikago.OptionSubscribe:
		for _, sub := range s.subs {
			if bytes.Equal(sub, vb) {
				// Already present
				return nil
			}
		}
		s.subs = append(s.subs, vb)
		return nil

	case pikago.OptionUnsubscribe:
		for i, sub := range s.subs {
			if bytes.Equal(sub, vb) {
				s.subs[i] = s.subs[len(s.subs)-1]
				s.subs = s.subs[:len(s.subs)-1]
				return nil
			}
		}
		// Subscription not present
		return pikago.ErrBadValue

	default:
		return pikago.ErrBadOption
	}
}

func (s *sub) GetOption(name string) (interface{}, error) {
	switch name {
	case pikago.OptionRaw:
		return s.raw, nil
	default:
		return nil, pikago.ErrBadOption
	}
}

// NewSocket allocates a new Socket using the SUB protocol.
func NewSocket() (pikago.Socket, error) {
	return pikago.MakeSocket(&sub{}), nil
}
