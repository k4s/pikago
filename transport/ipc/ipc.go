// Package ipc implements the IPC transport on top of UNIX domain sockets.
package ipc

import (
	"net"

	"github.com/k4s/pikago"
)

// options is used for shared GetOption/SetOption logic.
type options map[string]interface{}

// GetOption retrieves an option value.
func (o options) get(name string) (interface{}, error) {
	if o == nil {
		return nil, pikago.ErrBadOption
	}
	v, ok := o[name]
	if !ok {
		return nil, pikago.ErrBadOption
	}
	return v, nil
}

// SetOption sets an option.  We have none, so just ErrBadOption.
func (o options) set(string, interface{}) error {
	return pikago.ErrBadOption
}

type dialer struct {
	addr *net.UnixAddr
	sock pikago.Socket
	opts options
}

// Dial implements the PipeDialer Dial method
func (d *dialer) Dial() (pikago.Pipe, error) {

	conn, err := net.DialUnix("unix", nil, d.addr)
	if err != nil {
		return nil, err
	}
	return pikago.NewConnPipeIPC(conn, d.sock)
}

// SetOption implements a stub PipeDialer SetOption method.
func (d *dialer) SetOption(n string, v interface{}) error {
	return d.opts.set(n, v)
}

// GetOption implements a stub PipeDialer GetOption method.
func (d *dialer) GetOption(n string) (interface{}, error) {
	return d.opts.get(n)
}

type listener struct {
	addr     *net.UnixAddr
	sock     pikago.Socket
	listener *net.UnixListener
	opts     options
}

// Listen implements the PipeListener Listen method.
func (l *listener) Listen() error {
	listener, err := net.ListenUnix("unix", l.addr)
	if err != nil {
		return err
	}
	l.listener = listener
	return nil
}

func (l *listener) Address() string {
	return "ipc://" + l.addr.String()
}

// Accept implements the the PipeListener Accept method.
func (l *listener) Accept() (pikago.Pipe, error) {

	conn, err := l.listener.AcceptUnix()
	if err != nil {
		return nil, err
	}
	return pikago.NewConnPipeIPC(conn, l.sock)
}

// Close implements the PipeListener Close method.
func (l *listener) Close() error {
	l.listener.Close()
	return nil
}

// SetOption implements a stub PipeListener SetOption method.
func (l *listener) SetOption(n string, v interface{}) error {
	return l.opts.set(n, v)
}

// GetOption implements a stub PipeListener GetOption method.
func (l *listener) GetOption(n string) (interface{}, error) {
	return l.opts.get(n)
}

type ipcTran struct{}

// Scheme implements the Transport Scheme method.
func (t *ipcTran) Scheme() string {
	return "ipc"
}

// NewDialer implements the Transport NewDialer method.
func (t *ipcTran) NewDialer(addr string, sock pikago.Socket) (pikago.PipeDialer, error) {
	var err error

	if addr, err = pikago.StripScheme(t, addr); err != nil {
		return nil, err
	}

	d := &dialer{sock: sock, opts: nil}
	if d.addr, err = net.ResolveUnixAddr("unix", addr); err != nil {
		return nil, err
	}
	return d, nil
}

// NewListener implements the Transport NewListener method.
func (t *ipcTran) NewListener(addr string, sock pikago.Socket) (pikago.PipeListener, error) {
	var err error
	l := &listener{sock: sock}

	if addr, err = pikago.StripScheme(t, addr); err != nil {
		return nil, err
	}

	if l.addr, err = net.ResolveUnixAddr("unix", addr); err != nil {
		return nil, err
	}

	return l, nil
}

// NewTransport allocates a new IPC transport.
func NewTransport() pikago.Transport {
	return &ipcTran{}
}
