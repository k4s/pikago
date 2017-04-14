package pikago

//套接字是用于访问SP系统的主访问接口
//它是应用程序与消息传递拓扑结构的“connection”的抽象
//应用程序可以一次打开多个套接字
type Socket interface {
	Close() error

	Send([]byte) error

	Recv() ([]byte, error)

	// 就像Send(),不过SendMsg()应用了message headers.
	SendMsg(*Message) error

	// RecvMsg 接收一个完整的 message,包含 message header,
	//对于原始模式中的协议非常有用
	RecvMsg() (*Message, error)

	//Dial 拨号远程endpoint到Socket，开启一个异步goroutine维持建立的链接
	//如果重复拨号将返回错误
	Dial(addr string) error

	DialOptions(addr string, options map[string]interface{}) error

	// NewDialer 返回一个Dialer 接口对象
	NewDialer(addr string, options map[string]interface{}) (Dialer, error)

	//Listen 监听本地endpoint到Socket，如果远程拨号将开启一个一部goroutine维持
	//如果地址失效将返回一个错误
	Listen(addr string) error

	ListenOptions(addr string, options map[string]interface{}) error

	// NewListener 返回一个Listener 接口对象
	NewListener(addr string, options map[string]interface{}) (Listener, error)

	// GetOption 检索一个Socket选项
	GetOption(name string) (interface{}, error)

	// SetOption 设置一个Socket选项
	SetOption(name string, value interface{}) error

	// Protocol 获取当前协议
	GetProtocol() Protocol

	//添加一个新的Transport到Socket，
	//在此之前，传输特定的选项可能已经在传输中配置了
	AddTransport(Transport)

	//SetPortHook 设置一个PortHook 函数，当Port添加或者删除的时候调用
	SetPortHook(PortHook) PortHook
}
