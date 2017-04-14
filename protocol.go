package pikago

import (
	"time"
)

//Endpoint 表示协议实现对底层流传输的处理
//它可以被看作是TCP、IPC或其他类型的连接
type Endpoint interface {
	//GetID 返回一个唯一的31位Endpoint关联值
	GetID() uint32

	// Close 关闭节点
	Close() error

	//SendMsg 发送一个message，成功返回nil
	//这是一个阻塞调用
	SendMsg(*Message) error

	//RecvMsg 接收一个message，当发送错误，pipe将关闭，返回nil
	RecvMsg() *Message
}

// Protocol 实现protocol 处理接口，每个protocol类型将实现其中一个
//
type Protocol interface {

	//Init 由core模块调用，它还将保存一个handle供后面使用
	Init(ProtocolSocket)

	//Shutdown 用来消耗发送端,只有当socket被关闭干净，才会调用它
	//Protocols有linger time，徘徊消耗sokcets
	Shutdown(time.Time)

	//AddEndpoint添加一个新的Endpoint到socket
	//Endpoint 可能是一条connect，或者accept
	AddEndpoint(Endpoint)

	//RemoveEndpoint 从socket移除一个Endpoint，
	///Endpoint 可能是一条disconnected，或者关闭的connection
	RemoveEndpoint(Endpoint)

	// ProtocolNumber 返回一个16位的 protocol number
	Number() uint16

	// Name 返回一个name字符串
	Name() string

	// PeerNumber()返回16位number 给peer protocol
	PeerNumber() uint16

	// PeerName() 返回 name给peer protocol
	PeerName() string

	//GetOption用于检索的当前值一个选项
	//如果option没有当前值，返回一个EBadOption
	GetOption(string) (interface{}, error)

	//SetOption 设置option
	//如果选项不被许可，返回EBadOption
	//如果选择值没效，返回EBadValue
	SetOption(string, interface{}) error
}

//下面是一个协议可以选择实现的可选接口

//ProtocolRecvHook 协议的目的是成为一个额外的扩展接口
type ProtocolRecvHook interface {

	//RecvHook 调用在message被应用接收之前，message可能被修改，
	//如果返回false，说明message被删除了
	RecvHook(*Message) bool
}

//ProtocolSendHook 协议的目的是成为一个额外的扩展接口
type ProtocolSendHook interface {

	//SendHook 调用当应用调用发送message
	//如果返回false，说明message被删除了
	//当然，message也可能由于其它原因被删除
	SendHook(*Message) bool
}

//ProtocolSocket 是给protocols 和socket通讯的接口
//Protocol实现不应该访问任何sockets或pipes，除非使用函数作为许可在ProtocolSocket上面
//注意所有函数列表非阻塞
type ProtocolSocket interface {

	//SendChannel 应用注入messages到这里，然后protocol消费这些messages
	//当设置一个新的选项需要重新配置channel
	//(OptionWriteQLen)协议实现时,应再次调用这个函数来获取新channel的值
	SendChannel() <-chan *Message

	//RecvChannel 是channel用于接受message,protocol 注入messages到这里
	//最后由应用消费这些messages
	RecvChannel() chan<- *Message

	//protocol 等待这个channel关闭
	//当它是关闭的，表明应用程序已经关闭了读取的套接字，
	//该protocol应该停止对该实例的任何进一步读取操作
	CloseChannel() <-chan struct{}

	//GetOption 使用protocol检索选项在socket上面
	//最终调用的是socket的GetOption handler
	GetOption(string) (interface{}, error)

	//SetOption 使用Protocol设置选项在socket上面
	//注意的是，这可能设置transport选项,甚至回调到protocol的SetOption接口
	SetOption(string, interface{}) error

	// SetRecvError 可以迫使错误报告error，而不是等待message
	SetRecvError(error)

	// SetSendError 可以迫使错误报告error，而不是等待message
	SetSendError(error)
}

// 使用常量作为 protocol numbers.

const (
	ProtoPair       = (1 * 16)
	ProtoPub        = (2 * 16)
	ProtoSub        = (2 * 16) + 1
	ProtoReq        = (3 * 16)
	ProtoRep        = (3 * 16) + 1
	ProtoPush       = (5 * 16)
	ProtoPull       = (5 * 16) + 1
	ProtoSurveyor   = (6 * 16) + 2
	ProtoRespondent = (6 * 16) + 3
	ProtoBus        = (7 * 16)

	// 实验Protocols - 有使用风险

	ProtoStar = (100 * 16)
)

//ProtocolName 返回与给定协议号对应的名称
//这对于WebSocket这样的传输很有用，WebSocket使用的是文本名称而不是握手中的数字
func ProtocolName(number uint16) string {
	names := map[uint16]string{
		ProtoPair:       "pair",
		ProtoPub:        "pub",
		ProtoSub:        "sub",
		ProtoReq:        "req",
		ProtoRep:        "rep",
		ProtoPush:       "push",
		ProtoPull:       "pull",
		ProtoSurveyor:   "surveyor",
		ProtoRespondent: "respondent",
		ProtoBus:        "bus"}
	return names[number]
}

//ValidPeers 返回true，说明两个peer协议相同
//比如，REQ能连REP，不能和BUS
func ValidPeers(p1, p2 Protocol) bool {
	if p1.Number() != p2.PeerNumber() {
		return false
	}
	if p2.Number() != p1.PeerNumber() {
		return false
	}
	return true
}

//NullRecv简单循环,接收和丢弃消息,直到返回nil消息的端点
//这使得端点可以注意到一个断开的连接
//它的用途是仅供编写的协议使用——它使它们能够意识到即使没有数据发送时也会丢失连通性。
func NullRecv(ep Endpoint) {
	for {
		var m *Message
		if m = ep.RecvMsg(); m == nil {
			return
		}
		m.Free()
	}
}
