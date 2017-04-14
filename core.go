package pikago

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// defaultQLen是默认长度的 read/write 队列.
const defaultQLen = 128

// defaultMaxRwSize是默认最大的读写长度
const defaultMaxRwSize = 1024 * 1024

type socket struct {
	proto Protocol

	sync.Mutex

	wq       chan *Message // write queue
	wqLen    int           // write queue buffer length
	rq       chan *Message // read queue
	rqLen    int           // read queue buffer length
	closeq   chan struct{} // closed when user requests close
	recverrq chan struct{} // signaled when an error is pending

	closing    bool  // true if Socket was closed at API level
	active     bool  // true if either Dial or Listen has been successfully called
	bestEffort bool  // true if OptionBestEffort is set
	recverr    error // error to return on attempts to Recv()
	senderr    error // error to return on attempts to Send()

	rdeadline  time.Duration
	wdeadline  time.Duration
	reconntime time.Duration // reconnect time after error or disconnect
	reconnmax  time.Duration // max reconnect interval
	linger     time.Duration
	maxRwSize  int // max recv size

	pipes []*pipe

	listeners []*listener

	transports map[string]Transport

	// These are conditional "type aliases" for our self
	sendhook ProtocolSendHook
	recvhook ProtocolRecvHook

	// Port hook -- called when a port is added or removed
	porthook PortHook
}

func (sock *socket) addPipe(transport Pipe, d *dialer, l *listener) *pipe {
	p := newPipe(transport)
	p.d = d
	p.l = l

	if l == nil && d == nil {
		p.Close()
		return nil
	}

	sock.Lock()
	if fn := sock.porthook; fn != nil {
		sock.Unlock()
		if !fn(PortActionAdd, p) {
			p.Close()
			return nil
		}
		sock.Lock()
	}
	p.sock = sock
	p.index = len(sock.pipes)
	sock.pipes = append(sock.pipes, p)
	sock.Unlock()
	sock.proto.AddEndpoint(p)
	return p
}

func (sock *socket) remPipe(p *pipe) {
	sock.proto.RemoveEndpoint(p)
	sock.Lock()
	if p.index >= 0 {
		sock.pipes[p.index] = sock.pipes[len(sock.pipes)-1]
		sock.pipes[p.index].index = p.index
		sock.pipes = sock.pipes[:len(sock.pipes)-1]
		p.index = -1
	}
	sock.Unlock()
}

func newSocket(proto Protocol) *socket {
	sock := new(socket)
	sock.wqLen = defaultQLen
	sock.rqLen = defaultQLen
	sock.wq = make(chan *Message, sock.wqLen)
	sock.rq = make(chan *Message, sock.rqLen)
	sock.closeq = make(chan struct{})
	sock.recverrq = make(chan struct{})
	sock.reconntime = time.Millisecond * 100
	sock.reconnmax = time.Duration(0)
	sock.proto = proto
	sock.transports = make(map[string]Transport)
	sock.linger = time.Second
	sock.maxRwSize = defaultMaxRwSize

	// Add some conditionals now -- saves checks later
	if i, ok := interface{}(proto).(ProtocolRecvHook); ok {
		sock.recvhook = i.(ProtocolRecvHook)
	}
	if i, ok := interface{}(proto).(ProtocolSendHook); ok {
		sock.sendhook = i.(ProtocolSendHook)
	}

	proto.Init(sock)

	return sock
}

func MakeSocket(proto Protocol) Socket {
	return newSocket(proto)
}

func (sock *socket) SendChannel() <-chan *Message {
	sock.Lock()
	defer sock.Unlock()
	return sock.wq
}

func (sock *socket) RecvChannel() chan<- *Message {
	sock.Lock()
	defer sock.Unlock()
	return sock.rq
}

func (sock *socket) CloseChannel() <-chan struct{} {
	return sock.closeq
}

func (sock *socket) SetSendError(err error) {
	sock.Lock()
	sock.senderr = err
	sock.Unlock()
}

func (sock *socket) SetRecvError(err error) {
	sock.Lock()
	sock.recverr = err
	select {
	case sock.recverrq <- struct{}{}:
	default:
	}
	sock.Unlock()
}

func (sock *socket) Close() error {

	fin := time.Now().Add(sock.linger)

	DrainChannel(sock.wq, fin)

	sock.Lock()
	if sock.closing {
		sock.Unlock()
		return ErrClosed
	}
	sock.closing = true
	close(sock.closeq)

	for _, l := range sock.listeners {
		l.l.Close()
	}
	pipes := append([]*pipe{}, sock.pipes...)
	sock.Unlock()

	// 二次drain
	DrainChannel(sock.wq, fin)

	// 并告诉该protocol关闭和耗尽pipes
	sock.proto.Shutdown(fin)

	for _, p := range pipes {
		p.Close()
	}

	return nil
}

func (sock *socket) SendMsg(msg *Message) error {

	sock.Lock()
	e := sock.senderr
	if e != nil {
		sock.Unlock()
		return e
	}
	sock.Unlock()
	if sock.sendhook != nil {
		if ok := sock.sendhook.SendHook(msg); !ok {
			//静默放弃
			msg.Free()
			return nil
		}
	}
	sock.Lock()
	useBestEffort := sock.bestEffort
	if sock.wdeadline != 0 {
		msg.expire = time.Now().Add(sock.wdeadline)
	} else {
		msg.expire = time.Time{}
	}
	sock.Unlock()

	if !useBestEffort {
		timeout := mkTimer(sock.wdeadline)
		select {
		case <-timeout:
			return ErrSendTimeout
		case <-sock.closeq:
			return ErrClosed
		case sock.wq <- msg:
			return nil
		}
	} else {
		select {
		case <-sock.closeq:
			return ErrClosed
		case sock.wq <- msg:
			return nil
		default:
			msg.Free()
			return nil
		}
	}
}

func (sock *socket) Send(b []byte) error {
	msg := NewMessage(len(b))
	msg.Body = append(msg.Body, b...)
	return sock.SendMsg(msg)
}

func (sock *socket) String() string {
	return fmt.Sprintf("SOCKET[%s](%p)", sock.proto.Name(), sock)
}

func (sock *socket) RecvMsg() (*Message, error) {
	sock.Lock()
	timeout := mkTimer(sock.rdeadline)
	sock.Unlock()

	for {
		sock.Lock()
		if e := sock.recverr; e != nil {
			sock.Unlock()
			return nil, e
		}
		sock.Unlock()
		select {
		case <-timeout:
			return nil, ErrRecvTimeout
		case msg := <-sock.rq:
			if sock.recvhook != nil {
				if ok := sock.recvhook.RecvHook(msg); ok {
					return msg, nil
				} // else loop
				msg.Free()
			} else {
				return msg, nil
			}
		case <-sock.closeq:
			return nil, ErrClosed
		case <-sock.recverrq:
		}
	}
}

func (sock *socket) Recv() ([]byte, error) {
	msg, err := sock.RecvMsg()
	if err != nil {
		return nil, err
	}
	b := make([]byte, 0, len(msg.Body))
	b = append(b, msg.Body...)
	msg.Free()
	return b, nil
}

func (sock *socket) getTransport(addr string) Transport {
	var i int

	if i = strings.Index(addr, "://"); i < 0 {
		return nil
	}
	scheme := addr[:i]

	sock.Lock()
	defer sock.Unlock()

	t, ok := sock.transports[scheme]
	if t != nil && ok {
		return t
	}
	return nil
}

func (sock *socket) AddTransport(t Transport) {
	sock.Lock()
	sock.transports[t.Scheme()] = t
	sock.Unlock()
}

func (sock *socket) DialOptions(addr string, opts map[string]interface{}) error {

	d, err := sock.NewDialer(addr, opts)
	if err != nil {
		return err
	}
	return d.Dial()
}

func (sock *socket) Dial(addr string) error {
	return sock.DialOptions(addr, nil)
}

func (sock *socket) NewDialer(addr string, options map[string]interface{}) (Dialer, error) {
	var err error
	d := &dialer{sock: sock, addr: addr, closeq: make(chan struct{})}
	t := sock.getTransport(addr)
	if t == nil {
		return nil, ErrBadTran
	}
	if d.d, err = t.NewDialer(addr, sock); err != nil {
		return nil, err
	}
	for n, v := range options {
		if err = d.d.SetOption(n, v); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func (sock *socket) ListenOptions(addr string, options map[string]interface{}) error {
	l, err := sock.NewListener(addr, options)
	if err != nil {
		return err
	}
	//在这一步进行了监听
	if err = l.Listen(); err != nil {
		return err
	}
	return nil
}

func (sock *socket) Listen(addr string) error {
	return sock.ListenOptions(addr, nil)
}

func (sock *socket) NewListener(addr string, options map[string]interface{}) (Listener, error) {
	//这个函数设置一个goroutine接受入站连接
	//被接受的连接将被添加到一个被接受的连接列表中
	//Listener只是需要不断地侦听
	t := sock.getTransport(addr)
	if t == nil {
		return nil, ErrBadTran
	}
	var err error
	l := &listener{sock: sock, addr: addr}
	l.l, err = t.NewListener(addr, sock)
	if err != nil {
		return nil, err
	}
	for n, v := range options {
		if err = l.l.SetOption(n, v); err != nil {
			l.l.Close()
			return nil, err
		}
	}
	return l, nil
}

func (sock *socket) SetOption(name string, value interface{}) error {
	matched := false
	err := sock.proto.SetOption(name, value)
	if err == nil {
		matched = true
	} else if err != ErrBadOption {
		return err
	}
	switch name {
	case OptionRecvDeadline:
		sock.Lock()
		sock.rdeadline = value.(time.Duration)
		sock.Unlock()
		return nil
	case OptionSendDeadline:
		sock.Lock()
		sock.wdeadline = value.(time.Duration)
		sock.Unlock()
		return nil
	case OptionLinger:
		sock.Lock()
		sock.linger = value.(time.Duration)
		sock.Unlock()
		return nil
	case OptionWriteQLen:
		sock.Lock()
		defer sock.Unlock()
		if sock.active {
			return ErrBadOption
		}
		length := value.(int)
		if length < 0 {
			return ErrBadValue
		}
		owq := sock.wq
		sock.wqLen = length
		sock.wq = make(chan *Message, sock.wqLen)
		close(owq)
		return nil
	case OptionReadQLen:
		sock.Lock()
		defer sock.Unlock()
		if sock.active {
			return ErrBadOption
		}
		length := value.(int)
		if length < 0 {
			return ErrBadValue
		}
		sock.rqLen = length
		sock.rq = make(chan *Message, sock.rqLen)
		return nil
	case OptionMaxRecvSize:
		sock.Lock()
		defer sock.Unlock()
		switch value := value.(type) {
		case int:
			if value < 0 {
				return ErrBadValue
			}
			sock.maxRwSize = value
			return nil
		default:
			return ErrBadValue
		}
	case OptionReconnectTime:
		sock.Lock()
		sock.reconntime = value.(time.Duration)
		sock.Unlock()
		return nil
	case OptionMaxReconnectTime:
		sock.Lock()
		sock.reconnmax = value.(time.Duration)
		sock.Unlock()
		return nil
	case OptionBestEffort:
		sock.Lock()
		sock.bestEffort = value.(bool)
		sock.Unlock()
		return nil
	}
	if matched {
		return nil
	}
	return ErrBadOption
}

func (sock *socket) GetOption(name string) (interface{}, error) {
	val, err := sock.proto.GetOption(name)
	if err == nil {
		return val, nil
	}
	if err != ErrBadOption {
		return nil, err
	}

	switch name {
	case OptionRecvDeadline:
		sock.Lock()
		defer sock.Unlock()
		return sock.rdeadline, nil
	case OptionSendDeadline:
		sock.Lock()
		defer sock.Unlock()
		return sock.wdeadline, nil
	case OptionLinger:
		sock.Lock()
		defer sock.Unlock()
		return sock.linger, nil
	case OptionWriteQLen:
		sock.Lock()
		defer sock.Unlock()
		return sock.wqLen, nil
	case OptionReadQLen:
		sock.Lock()
		defer sock.Unlock()
		return sock.rqLen, nil
	case OptionMaxRecvSize:
		sock.Lock()
		defer sock.Unlock()
		return sock.maxRwSize, nil
	case OptionReconnectTime:
		sock.Lock()
		defer sock.Unlock()
		return sock.reconntime, nil
	case OptionMaxReconnectTime:
		sock.Lock()
		defer sock.Unlock()
		return sock.reconnmax, nil
	}
	return nil, ErrBadOption
}

func (sock *socket) GetProtocol() Protocol {
	return sock.proto
}

func (sock *socket) SetPortHook(newhook PortHook) PortHook {
	sock.Lock()
	oldhook := sock.porthook
	sock.porthook = newhook
	sock.Unlock()
	return oldhook
}

type dialer struct {
	d      PipeDialer
	sock   *socket
	addr   string
	closed bool
	active bool
	closeq chan struct{}
}

func (d *dialer) Dial() error {
	d.sock.Lock()
	if d.active {
		d.sock.Unlock()
		return ErrAddrInUse
	}
	d.closeq = make(chan struct{})
	d.sock.active = true
	d.active = true
	d.sock.Unlock()
	go d.dialer()
	return nil
}

func (d *dialer) Close() error {
	d.sock.Lock()
	if d.closed {
		d.sock.Unlock()
		return ErrClosed
	}
	d.closed = true
	close(d.closeq)
	d.sock.Unlock()
	return nil
}

func (d *dialer) GetOption(n string) (interface{}, error) {
	return d.d.GetOption(n)
}

func (d *dialer) SetOption(n string, v interface{}) error {
	return d.d.SetOption(n, v)
}

func (d *dialer) Address() string {
	return d.addr
}

//dialer是用来dial或从goroutine重拨。
func (d *dialer) dialer() {
	rtime := d.sock.reconntime
	rtmax := d.sock.reconnmax
	for {
		p, err := d.d.Dial()
		if err == nil {
			// reset retry time
			rtime = d.sock.reconntime
			d.sock.Lock()
			if d.closed {
				p.Close()
				return
			}
			d.sock.Unlock()
			if cp := d.sock.addPipe(p, d, nil); cp != nil {
				select {
				case <-d.sock.closeq: // parent socket closed
				case <-cp.closeq: // disconnect event
				case <-d.closeq: // dialer closed
				}
			}
		}

		// 这里重拨
		select {
		case <-d.closeq: // dialer closed
			return
		case <-d.sock.closeq: // exit if parent socket closed
			return
		case <-time.After(rtime):
			if rtmax > 0 {
				rtime *= 2
				if rtime > rtmax {
					rtime = rtmax
				}
			}
			continue
		}
	}
}

type listener struct {
	l    PipeListener
	sock *socket
	addr string
}

func (l *listener) GetOption(n string) (interface{}, error) {
	return l.l.GetOption(n)
}

func (l *listener) SetOption(n string, v interface{}) error {
	return l.l.SetOption(n, v)
}

// serve 循环调用Accept routine.
func (l *listener) serve() {
	for {
		select {
		case <-l.sock.closeq:
			return
		default:
		}

		//如果底层的管道侦听器关闭，或者不监听，返回一个错误
		if pipe, err := l.l.Accept(); err == nil {
			l.sock.addPipe(pipe, nil, l)
		} else if err == ErrClosed {
			return
		}
	}
}

func (l *listener) Listen() error {
	//这个函数设置一个goroutine接受入站连接
	//被接受的连接将被添加到一个被接受的连接列表中。侦听器只是需要不断地侦听，而不受限制。

	if err := l.l.Listen(); err != nil {
		return err
	}
	l.sock.Lock()
	l.sock.listeners = append(l.sock.listeners, l)
	l.sock.active = true
	l.sock.Unlock()
	go l.serve()
	return nil
}

func (l *listener) Address() string {
	return l.l.Address()
}

func (l *listener) Close() error {
	return l.l.Close()
}
