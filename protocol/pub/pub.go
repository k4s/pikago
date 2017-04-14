package pub

import (
	"sync"
	"time"

	"github.com/k4s/pikago"
)

type pub struct {
	sock pikago.ProtocolSocket
	eps  map[uint32]*pubEp
	raw  bool
	w    pikago.Waiter
	sync.Mutex
}

type pubEp struct {
	ep pikago.Endpoint
	q  chan *pikago.Message
	p  *pub
	w  pikago.Waiter
}

func (p *pub) Init(sock pikago.ProtocolSocket) {
	p.sock = sock
	p.eps = make(map[uint32]*pubEp)
	p.sock.SetRecvError(pikago.ErrProtoOp)
	p.w.Init()
	p.w.Add()
	go p.sender()
}

func (p *pub) Shutdown(expire time.Time) {

	p.w.WaitAbsTimeout(expire)

	p.Lock()
	peers := p.eps
	p.eps = make(map[uint32]*pubEp)
	p.Unlock()

	for id, peer := range peers {
		pikago.DrainChannel(peer.q, expire)
		close(peer.q)
		delete(peers, id)
	}
}

// Bottom sender.
func (pe *pubEp) peerSender() {

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

// Top sender.
func (p *pub) sender() {
	defer p.w.Done()

	cq := p.sock.CloseChannel()
	sq := p.sock.SendChannel()

	for {
		select {
		case <-cq:
			return

		case m := <-sq:
			if m == nil {
				sq = p.sock.SendChannel()
				continue
			}

			p.Lock()
			for _, peer := range p.eps {
				m := m.Dup()
				select {
				case peer.q <- m:
				default:
					m.Free()
				}
			}
			p.Unlock()
			m.Free()
		}
	}
}

func (p *pub) AddEndpoint(ep pikago.Endpoint) {
	depth := 16
	if i, err := p.sock.GetOption(pikago.OptionWriteQLen); err == nil {
		depth = i.(int)
	}
	pe := &pubEp{ep: ep, p: p, q: make(chan *pikago.Message, depth)}
	pe.w.Init()
	p.Lock()
	p.eps[ep.GetID()] = pe
	p.Unlock()

	pe.w.Add()
	go pe.peerSender()
	go pikago.NullRecv(ep)
}

func (p *pub) RemoveEndpoint(ep pikago.Endpoint) {
	id := ep.GetID()
	p.Lock()
	pe := p.eps[id]
	delete(p.eps, id)
	p.Unlock()
	if pe != nil {
		close(pe.q)
	}
}

func (*pub) Number() uint16 {
	return pikago.ProtoPub
}

func (*pub) PeerNumber() uint16 {
	return pikago.ProtoSub
}

func (*pub) Name() string {
	return "pub"
}

func (*pub) PeerName() string {
	return "sub"
}

func (p *pub) SetOption(name string, v interface{}) error {
	var ok bool
	switch name {
	case pikago.OptionRaw:
		if p.raw, ok = v.(bool); !ok {
			return pikago.ErrBadValue
		}
		return nil
	default:
		return pikago.ErrBadOption
	}
}

func (p *pub) GetOption(name string) (interface{}, error) {
	switch name {
	case pikago.OptionRaw:
		return p.raw, nil
	default:
		return nil, pikago.ErrBadOption
	}
}

// NewSocket allocates a new Socket using the PUB protocol.
func NewSocket() (pikago.Socket, error) {
	return pikago.MakeSocket(&pub{}), nil
}
