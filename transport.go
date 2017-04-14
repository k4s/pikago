package pikago

import (
	"net"
	"strings"
)

//Pipe像一个全双工消息在两个peer之间传输的介质
//
//Pipe仅供传输实现者使用，不应该直接在应用程序中使用
type Pipe interface {

	//发送一个massage，如果Pipe是关闭的，将返回一个error
	Send(*Message) error

	//接收一个message,如果信息不完整或者Pipe是关闭的，将返回一个error
	//为了减轻DOS攻击，限制最大message长度为1M
	Recv() (*Message, error)

	//关闭一个transport，如果连续关闭将返回错误，transport在关闭的时候，缓冲区可能还仍然保存有远程peer的message
	Close() error

	//LocalProtocol 返回一个16字节的SP protocol number，在正常连接完成后发送
	LocalProtocol() uint16

	//RemoteProtocol  返回一个16字节的SP protocol number，在正常连接完成后发送
	RemoteProtocol() uint16

	// IsOpen 返回true，说明当前pipe是活跃的
	IsOpen() bool

	//GetProp 返回一个传输特定属性
	//这些像是配置options，但是是一些特殊只读在单连接上
	//如果property不存在，那将返回ErrBadProperty
	GetProp(string) (interface{}, error)
}

//PipeDialer 代表一个客户端初始化连接
//
//PipeDialer只是用于transport实现,不应直接用于应用程序
type PipeDialer interface {
	//Dial初始化远程peer连接
	Dial() (Pipe, error)

	//SetOption 设置本地配置在dialer上
	//ErrBadOption 返回说明配置不被认可
	//ErrBadValue 返回说明配置的值类型不正确
	SetOption(name string, value interface{}) error

	//GetOption 获取本地配置在dialer上
	//ErrBadOption 返回说明配置不被认可
	GetOption(name string) (value interface{}, err error)
}

//PipeListener 代表一个服务端初始化连接
//
//PipeListener 只是用于transport实现,不应直接用于应用程序
type PipeListener interface {

	//在Accept()调用之前调用
	//这等价于socket里面的bind()+listen()
	Listen() error

	//服务端建立初始化来自客户端的拨号请求
	Accept() (Pipe, error)

	//
	Close() error

	//SetOption 设置本地配置在dialer上
	//ErrBadOption 返回说明配置不被认可
	//ErrBadValue 返回说明配置的值类型不正确
	SetOption(name string, value interface{}) error

	//GetOption 获取本地配置在listener上
	//ErrBadOption 返回说明配置不被认可
	GetOption(name string) (value interface{}, err error)

	//获取本地地址，在调用Listen()之后调用
	Address() string
}

//Transport 是transport的接口实现
type Transport interface {
	// Scheme 返回一个SP "addresses"前缀，比如：
	// "tcp" (for "tcp://xxx..."), "ipc", "inproc", etc.
	Scheme() string

	//NewDialer 在当前Transport创建一个新的Dialer
	NewDialer(url string, sock Socket) (PipeDialer, error)

	//NewListener 在当前Transport创建一个新的Listener
	// This generally also arranges for an OS-level file descriptor to be
	// opened, and bound to the the given address, as well as establishing
	// any "listen" backlog.
	NewListener(url string, sock Socket) (PipeListener, error)
}

// StripScheme 剥夺scheme，返回地址
func StripScheme(t Transport, addr string) (string, error) {
	if !strings.HasPrefix(addr, t.Scheme()+"://") {
		return addr, ErrBadTran
	}
	return addr[len(t.Scheme()+"://"):], nil
}

//ResolveTCPAddr就像net.ResolveTCPAddr,但处理nanomsg URLs中使用的通配符
//以一个空字符串,表明所有的本地接口
func ResolveTCPAddr(addr string) (*net.TCPAddr, error) {
	if strings.HasPrefix(addr, "*") {
		addr = addr[1:]
	}
	return net.ResolveTCPAddr("tcp", addr)
}
