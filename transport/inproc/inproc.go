// Package inproc implements an simple inproc transport for pikago.
package inproc

import (
	"strings"
	"sync"

	"github.com/k4s/pikago"
)

// inproc implements the Pipe interface on top of channels.
type inproc struct {
	rq     chan *pikago.Message
	wq     chan *pikago.Message
	closeq chan struct{}
	readyq chan struct{}
	proto  pikago.Protocol
	addr   addr
	peer   *inproc
}

type addr string

func (a addr) String() string {
	s := string(a)
	if strings.HasPrefix(s, "inproc://") {
		s = s[len("inproc://"):]
	}
	return s
}

func (addr) Network() string {
	return "inproc"
}

type listener struct {
	addr      string
	proto     pikago.Protocol
	accepters []*inproc
}

type inprocTran struct{}

var listeners struct {
	// Who is listening, on which "address"?
	byAddr map[string]*listener
	cv     sync.Cond
	mx     sync.Mutex
}

func init() {
	listeners.byAddr = make(map[string]*listener)
	listeners.cv.L = &listeners.mx
}

func (p *inproc) Recv() (*pikago.Message, error) {

	if p.peer == nil {
		return nil, pikago.ErrClosed
	}
	select {
	case m, ok := <-p.rq:
		if m == nil || !ok {
			return nil, pikago.ErrClosed
		}
		// Upper protocols expect to have to pick header and
		// body part.  So mush them back together.
		//msg.Body = append(msg.Header, msg.Body...)
		//msg.Header = make([]byte, 0, 32)
		return m, nil
	case <-p.closeq:
		return nil, pikago.ErrClosed
	}
}

func (p *inproc) Send(m *pikago.Message) error {

	if p.peer == nil {
		return pikago.ErrClosed
	}

	if m.Expired() {
		m.Free()
		return nil
	}

	// Upper protocols expect to have to pick header and body part.
	// Also we need to have a fresh copy of the message for receiver, to
	// break ownership.
	nmsg := pikago.NewMessage(len(m.Header) + len(m.Body))
	nmsg.Body = append(nmsg.Body, m.Header...)
	nmsg.Body = append(nmsg.Body, m.Body...)
	select {
	case p.wq <- nmsg:
		return nil
	case <-p.closeq:
		nmsg.Free()
		return pikago.ErrClosed
	}
}

func (p *inproc) LocalProtocol() uint16 {
	return p.proto.Number()
}

func (p *inproc) RemoteProtocol() uint16 {
	return p.proto.PeerNumber()
}

func (p *inproc) Close() error {
	close(p.closeq)
	return nil
}

func (p *inproc) IsOpen() bool {
	select {
	case <-p.closeq:
		return false
	default:
		return true
	}
}

func (p *inproc) GetProp(name string) (interface{}, error) {
	switch name {
	case pikago.PropRemoteAddr:
		return p.addr, nil
	case pikago.PropLocalAddr:
		return p.addr, nil
	}
	// We have no special properties
	return nil, pikago.ErrBadProperty
}

type dialer struct {
	addr  string
	proto pikago.Protocol
}

func (d *dialer) Dial() (pikago.Pipe, error) {

	var server *inproc
	client := &inproc{proto: d.proto, addr: addr(d.addr)}
	client.readyq = make(chan struct{})
	client.closeq = make(chan struct{})

	listeners.mx.Lock()

	// NB: No timeouts here!
	for {
		var l *listener
		var ok bool
		if l, ok = listeners.byAddr[d.addr]; !ok || l == nil {
			listeners.mx.Unlock()
			return nil, pikago.ErrConnRefused
		}

		if !pikago.ValidPeers(client.proto, l.proto) {
			return nil, pikago.ErrBadProto
		}

		if len(l.accepters) != 0 {
			server = l.accepters[len(l.accepters)-1]
			l.accepters = l.accepters[:len(l.accepters)-1]
			break
		}

		listeners.cv.Wait()
		continue
	}

	listeners.mx.Unlock()

	server.wq = make(chan *pikago.Message)
	server.rq = make(chan *pikago.Message)
	client.rq = server.wq
	client.wq = server.rq
	server.peer = client
	client.peer = server

	close(server.readyq)
	close(client.readyq)
	return client, nil
}

func (*dialer) SetOption(string, interface{}) error {
	return pikago.ErrBadOption
}

func (*dialer) GetOption(string) (interface{}, error) {
	return nil, pikago.ErrBadOption
}

func (l *listener) Listen() error {
	listeners.mx.Lock()
	if _, ok := listeners.byAddr[l.addr]; ok {
		listeners.mx.Unlock()
		return pikago.ErrAddrInUse
	}
	listeners.byAddr[l.addr] = l
	listeners.cv.Broadcast()
	listeners.mx.Unlock()
	return nil
}

func (l *listener) Address() string {
	return l.addr
}

func (l *listener) Accept() (pikago.Pipe, error) {
	server := &inproc{proto: l.proto, addr: addr(l.addr)}
	server.readyq = make(chan struct{})
	server.closeq = make(chan struct{})

	listeners.mx.Lock()
	l.accepters = append(l.accepters, server)
	listeners.cv.Broadcast()
	listeners.mx.Unlock()

	select {
	case <-server.readyq:
		return server, nil
	case <-server.closeq:
		return nil, pikago.ErrClosed
	}
}

func (*listener) SetOption(string, interface{}) error {
	return pikago.ErrBadOption
}

func (*listener) GetOption(string) (interface{}, error) {
	return nil, pikago.ErrBadOption
}

func (l *listener) Close() error {
	listeners.mx.Lock()
	if listeners.byAddr[l.addr] == l {
		delete(listeners.byAddr, l.addr)
	}
	servers := l.accepters
	l.accepters = nil
	listeners.cv.Broadcast()
	listeners.mx.Unlock()

	for _, s := range servers {
		close(s.closeq)
	}

	return nil
}

func (t *inprocTran) Scheme() string {
	return "inproc"
}

func (t *inprocTran) NewDialer(addr string, sock pikago.Socket) (pikago.PipeDialer, error) {
	if _, err := pikago.StripScheme(t, addr); err != nil {
		return nil, err
	}
	return &dialer{addr: addr, proto: sock.GetProtocol()}, nil
}

func (t *inprocTran) NewListener(addr string, sock pikago.Socket) (pikago.PipeListener, error) {
	if _, err := pikago.StripScheme(t, addr); err != nil {
		return nil, err
	}
	l := &listener{addr: addr, proto: sock.GetProtocol()}
	return l, nil
}

// NewTransport allocates a new inproc:// transport.
func NewTransport() pikago.Transport {
	return &inprocTran{}
}
