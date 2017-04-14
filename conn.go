package pikago

import "net"
import "encoding/binary"
import "io"

//conn 实现Pipe接口
type conn struct {
	c     net.Conn
	proto Protocol
	sock  Socket
	open  bool
	props map[string]interface{}
	maxrw int64
}

//connipc 几乎像是一条正常的conn,但IPC协议坚持填充头字节(值为 1)的messages
//这是为了兼容nanomsg——值不能是1
type connipc struct {
	conn
}

func (p *conn) Recv() (*Message, error) {
	var sz int64
	var err error
	var msg *Message

	if err = binary.Read(p.c, binary.BigEndian, &sz); err != nil {
		return nil, err
	}

	if sz < 0 || (p.maxrw > 0 && sz > p.maxrw) {
		return nil, ErrTooLong
	}
	msg = NewMessage(int(sz))
	msg.Body = msg.Body[0:sz]
	if _, err = io.ReadFull(p.c, msg.Body); err != nil {
		msg.Free()
		return nil, err
	}
	return msg, nil
}

func (p *conn) Send(msg *Message) error {
	sz := uint64(len(msg.Header) + len(msg.Body))

	if msg.Expired() {
		msg.Free()
		return nil
	}

	//写入头长度
	if err := binary.Write(p.c, binary.BigEndian, sz); err != nil {
		return err
	}
	if _, err := p.c.Write(msg.Header); err != nil {
		return err
	}
	if _, err := p.c.Write(msg.Body); err != nil {
		return err
	}
	msg.Free()
	return nil
}

func (p *conn) LocalProtocol() uint16 {
	return p.proto.Number()
}

// RemoteProtocol returns  peer's protocol number.
func (p *conn) RemoteProtocol() uint16 {
	return p.proto.PeerNumber()
}

func (p *conn) Close() error {
	p.open = false
	return p.c.Close()
}

func (p *conn) IsOpen() bool {
	return p.open
}

func (p *conn) GetProp(n string) (interface{}, error) {
	if v, ok := p.props[n]; ok {
		return v, nil
	}
	return nil, ErrBadProperty
}

//NewConnPipe 使用net.Conn提供分配一个新的Pipe，并将其初始化。
//它执行SP层所需的握手，只有在SP层执行完成后才返回管道。
//
//面向流的传输可以利用这个来实现传输。实现还需要实现PipeDialer PipeAccepter,Transport结构
//使用这个分层的接口，一旦建立了底层连接，实现就不必为传递实际的SP消息而烦恼了
func NewConnPipe(c net.Conn, sock Socket, props ...interface{}) (Pipe, error) {
	p := &conn{c: c, proto: sock.GetProtocol(), sock: sock}

	if err := p.handshake(props); err != nil {
		return nil, err
	}

	return p, nil
}

// NewConnPipeIPC分配一个新的Pipe使用IPC交换协议。
func NewConnPipeIPC(c net.Conn, sock Socket, props ...interface{}) (Pipe, error) {
	p := &connipc{conn: conn{c: c, proto: sock.GetProtocol(), sock: sock}}

	if err := p.handshake(props); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *connipc) Send(msg *Message) error {

	l := uint64(len(msg.Header) + len(msg.Body))
	one := [1]byte{1}
	var err error

	// send length header
	if _, err = p.c.Write(one[:]); err != nil {
		return err
	}
	if err = binary.Write(p.c, binary.BigEndian, l); err != nil {
		return err
	}
	if _, err = p.c.Write(msg.Header); err != nil {
		return err
	}

	if _, err = p.c.Write(msg.Body); err != nil {
		return err
	}
	msg.Free()
	return nil
}

func (p *connipc) Recv() (*Message, error) {

	var sz int64
	var err error
	var msg *Message
	var one [1]byte

	if _, err = p.c.Read(one[:]); err != nil {
		return nil, err
	}
	if err = binary.Read(p.c, binary.BigEndian, &sz); err != nil {
		return nil, err
	}

	//限制messages最大接收值,这避免了潜在的拒绝服务
	if sz < 0 || (p.maxrw > 0 && sz > p.maxrw) {
		return nil, ErrTooLong
	}
	msg = NewMessage(int(sz))
	msg.Body = msg.Body[0:sz]
	if _, err = io.ReadFull(p.c, msg.Body); err != nil {
		msg.Free()
		return nil, err
	}
	return msg, nil
}

type connHeader struct {
	Zero    byte // must be zero
	S       byte // 'S'
	P       byte // 'P'
	Version byte // only zero at present
	Proto   uint16
	Rsvd    uint16 // always zero at present
}

func (p *conn) handshake(props []interface{}) error {
	var err error

	p.props = make(map[string]interface{})
	p.props[PropLocalAddr] = p.c.LocalAddr()
	p.props[PropRemoteAddr] = p.c.RemoteAddr()

	for len(props) >= 2 {
		switch name := props[0].(type) {
		case string:
			p.props[name] = props[1]
		default:
			return ErrBadProperty
		}
		props = props[2:]
	}

	if v, e := p.sock.GetOption(OptionMaxRecvSize); e == nil {
		//socket保证这是一个整数
		p.maxrw = int64(v.(int))
	}

	h := connHeader{S: 'S', P: 'P', Proto: p.proto.Number()}
	if err = binary.Write(p.c, binary.BigEndian, &h); err != nil {
		return err
	}
	if err = binary.Read(p.c, binary.BigEndian, &h); err != nil {
		p.c.Close()
		return err
	}
	if h.Zero != 0 || h.S != 'S' || h.P != 'P' || h.Rsvd != 0 {
		p.c.Close()
		return ErrBadHeader
	}

	//目前唯一支持的版本号是“0”,at offset 3.
	if h.Version != 0 {
		p.c.Close()
		return ErrBadVersion
	}

	//协议号 16位(big-endian) at offset 4.
	if h.Proto != p.proto.PeerNumber() {
		p.c.Close()
		return ErrBadProto
	}
	p.open = true
	return nil
}
