package pikago

//Port表示高级通信channel的高级接口。例如，有一个与给定的TCP连接相关联的连接。
//此接口用于应用程序使用。
//
// 注意,作用不能直接发送或接收数据端口。
type Port interface {

	//Address返回与该port相关联的地址(URL表单)
	//这个匹配的字符串传递给Dial()或Listen()。
	Address() string

	//GetProp 返回一个interface{}的属性,对于不同的传输类型，值会有所不同
	GetProp(name string) (interface{}, error)

	// IsOpen
	IsOpen() bool

	//Close 关闭Conn，断开连接，
	//注意，如果一个dialer出现并激活，它就会重新拨号
	Close() error

	// IsServer 返回 true 说明连接是 server (Listen).
	IsServer() bool

	// IsClient 返回 true 说明连接是 client (Dial).
	IsClient() bool

	// LocalProtocol 返回 local protocol number.
	LocalProtocol() uint16

	// RemoteProtocol 返回 remote protocol number.
	RemoteProtocol() uint16

	//Dialer 返回该Port的dialer，如果服务器，则返回nil
	Dialer() Dialer

	// Listener  返回该Port的listener, 如果客户端，则返回nil
	Listener() Listener
}

// PortAction 确定Port上的操作是添加还是删除。
type PortAction int

// PortAction 值.
const (
	PortActionAdd = iota
	PortActionRemove
)

//PortHook是调用一个函数,当添加或删除一个port或从一个socket
//PortActionAdd的情况下,该函数会返回false,表明没有被添加
type PortHook func(PortAction, Port) bool
