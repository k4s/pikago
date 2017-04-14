package tcp

import (
	"net"

	"github.com/k4s/pikago"
)

// options is used for shared GetOption/SetOption logic.
type options map[string]interface{}

// GetOption retrieves an option value.
func (o options) get(name string) (interface{}, error) {
	v, ok := o[name]
	if !ok {
		return nil, pikago.ErrBadOption
	}
	return v, nil
}

// SetOption sets an option.
func (o options) set(name string, val interface{}) error {
	switch name {
	case pikago.OptionNoDelay:
		fallthrough
	case pikago.OptionKeepAlive:
		switch v := val.(type) {
		case bool:
			o[name] = v
			return nil
		default:
			return pikago.ErrBadValue
		}
	}
	return pikago.ErrBadOption
}

func newOptions() options {
	o := make(map[string]interface{})
	o[pikago.OptionNoDelay] = true
	o[pikago.OptionKeepAlive] = true
	return options(o)
}

func (o options) configTCP(conn *net.TCPConn) error {
	if v, ok := o[pikago.OptionNoDelay]; ok {
		if err := conn.SetNoDelay(v.(bool)); err != nil {
			return err
		}
	}
	if v, ok := o[pikago.OptionKeepAlive]; ok {
		if err := conn.SetKeepAlive(v.(bool)); err != nil {
			return err
		}
	}
	return nil
}

type dialer struct {
	addr *net.TCPAddr
	sock pikago.Socket
	opts options
}

func (d *dialer) Dial() (pikago.Pipe, error) {
	conn, err := net.DialTCP("tcp", nil, d.addr)
	if err != nil {
		return nil, err
	}
	if err = d.opts.configTCP(conn); err != nil {
		conn.Close()
		return nil, err
	}
	return pikago.NewConnPipe(conn, d.sock)
}

func (d *dialer) SetOption(n string, v interface{}) error {
	return d.opts.set(n, v)
}

func (d *dialer) GetOption(n string) (interface{}, error) {
	return d.opts.get(n)
}

type listener struct {
	addr     *net.TCPAddr
	bound    net.Addr
	sock     pikago.Socket
	listener *net.TCPListener
	opts     options
}

func (l *listener) Listen() (err error) {
	l.listener, err = net.ListenTCP("tcp", l.addr)
	if err == nil {
		l.bound = l.listener.Addr()
	}
	return
}

func (l *listener) Accept() (pikago.Pipe, error) {

	if l.listener == nil {
		return nil, pikago.ErrClosed
	}
	conn, err := l.listener.AcceptTCP()
	if err != nil {
		return nil, err
	}
	if err = l.opts.configTCP(conn); err != nil {
		conn.Close()
		return nil, err
	}
	return pikago.NewConnPipe(conn, l.sock)
}

func (l *listener) Address() string {
	if b := l.bound; b != nil {
		return "tcp://" + b.String()
	}
	return "tcp://" + l.addr.String()
}

func (l *listener) Close() error {
	l.listener.Close()
	return nil
}

func (l *listener) SetOption(n string, v interface{}) error {
	return l.opts.set(n, v)
}

func (l *listener) GetOption(n string) (interface{}, error) {
	return l.opts.get(n)
}

type tcpTransport struct {
	// localAddr net.Addr
}

func (t *tcpTransport) Scheme() string {
	return "tcp"
}

func (t *tcpTransport) NewDialer(addr string, sock pikago.Socket) (pikago.PipeDialer, error) {
	var err error
	d := &dialer{sock: sock, opts: newOptions()}

	if addr, err = pikago.StripScheme(t, addr); err != nil {
		return nil, err
	}

	if d.addr, err = pikago.ResolveTCPAddr(addr); err != nil {
		return nil, err
	}
	return d, nil
}

func (t *tcpTransport) NewListener(addr string, sock pikago.Socket) (pikago.PipeListener, error) {
	var err error
	l := &listener{sock: sock, opts: newOptions()}

	if addr, err = pikago.StripScheme(t, addr); err != nil {
		return nil, err
	}

	if l.addr, err = pikago.ResolveTCPAddr(addr); err != nil {
		return nil, err
	}

	return l, nil
}

// NewTransport allocates a new TCP transport.
func NewTransport() pikago.Transport {
	return &tcpTransport{}
}
