package pikago

//以下是选择使用SetOption GetOption

const (

	//OptionRaw用于启用 RAW mode处理
	//不同于普通模式的细节不同，从协议到协议的细节也各不相同
	// RAW mode对应于AF_SP_RAW C的变体,并且必须使用Devices
	// RAW mode sockets 是完全无状态，任何状态之间recv/send消息包含在消息头
	//Protocol名称从“X”默认为RAW mode相同的协议没有领先的“X”。
	//传递的值是一种bool。
	OptionRaw = "RAW"

	//OptionRecvDeadline 下次Recv的超时时间，值是一个 time.Duration.
	//可以通过零值来表示不应该应用任何超时,负值表示非阻塞操作
	//默认情况下，没有超时。
	OptionRecvDeadline = "RECV-DEADLINE"

	//OptionSendDeadline 是下一个发送超时的时间，值是一个time.Duration
	//可以通过零值来表示不应该应用任何超时,负值表示非阻塞操作
	//默认情况下，没有超时。
	OptionSendDeadline = "SEND-DEADLINE"

	//OptionRetryTime 用于REQ。这个值是time.Duration.
	//当请求在给定的时间内没有被回复时，请求会自动地对一个可用的对等点产生resent。
	//这个值应该比最大可能的处理和传输时间长
	//值为0表示不应该发送自动重试。默认值为1分钟。
	//
	//注意的是，更改此选项只会影响在选项设置后发送的请求。更改值时，请求未完成可能不会产生预期的效果。
	OptionRetryTime = "RETRY-TIME"

	//OptionSubscribe 用于 SUB/XSUB. 参数是[]byte.
	//应用程序将接收以这个前缀开头的消息
	//多个订阅可能对一个给定的socket生效。该应用程序将不会收到与当前订阅不匹配的消息。
	//(如果没有SUB/XSUB 订阅socket,应用程序将不会收到任何消息。可以使用空前缀来订阅所有消息。)
	OptionSubscribe = "SUBSCRIBE"

	//OptionUnsubscribe 用于SUB/XSUB ，参数是[]byte.
	//表示先前建立的订阅,从socket中删除。
	OptionUnsubscribe = "UNSUBSCRIBE"

	//OptionSurveyTime是用来表示survey结果的最后期限,当使用SURVEYOR socket时，在此之后的消息将被丢弃
	//设置OptionRecvDeadline开始调查的时候,尝试接收消息失败ErrRecvTimeout调查结束
	//值是一个time.Duration。0可以被传递来表示无限的时间。默认值是1秒。
	OptionSurveyTime = "SURVEY-TIME"

	//OptionTLSConfig 用于供应TLS配置细节
	//它可以使用ListenOptions或DialOptions来设置,该参数是一个tls.Config pointer.
	OptionTLSConfig = "TLS-CONFIG"

	//OptionWriteQLen用于设置大小,写队列消息的channel
	//默认情况下，它是128。如果在socket上调用了Dial或Listen，则无法设置此选项
	OptionWriteQLen = "WRITEQ-LEN"

	//OptionReadQLen用于设置大小,读取队列的消息channel
	//默认情况下，它是128。如果在socket上调用了Dial或Listen，则无法设置此选项
	OptionReadQLen = "READQ-LEN"

	//OptionKeepAlive用于设置TCP KeepAlive。Value是一个布尔值
	//默认是true
	OptionKeepAlive = "KEEPALIVE"

	//OptionNoDelay用于配置Nagle,当真正的消息被发送尽,否则可能会出现一些缓冲
	//Value是一个布尔值。默认是正确的。
	OptionNoDelay = "NO-DELAY"

	//OptionLinger用于设置linger property。这是在调用Close()时等待发送队列耗尽的时间
	//Close()可能会阻止了这个时长如果有未寄出的数据,就返回所有数据交付给transport。
	//价值是一个time.Duration.。默认是1秒。
	OptionLinger = "LINGER"

	//OptionTTL用于设置messages的最大生存时间。
	//请注意，并不是所有的协议都能在这个时候实现这时间点，但是对于那些确实如此的协议来说，如果消息传递的内容超过了这许多设备，那么它就会被删除。
	//这用于在拓扑中提供对循环的保护。缺省情况是特定于协议的。
	OptionTTL = "TTL"

	//OptionMaxRecvSize 支持入站消息的最大接收大小
	//因为协议允许发送方指定传入消息的大小,如果规模过于庞大,远程可以执行远程拒绝服务请求大得离谱,在发送消息大小,然后停滞不前
	//默认值是1MB。
	//
	//值为0的值消除了限制，但不应该使用，除非绝对确信对等方是可信任的。
	//
	//并不是所有的transports都尊重这个限制。例如，当使用inproc时，这个限制是没有意义的。
	//
	//
	//请注意，该大小包含任何协议特定的头部。所以应该选择一个有点太大，而不是太小的值
	//
	//此选项仅用于防止系统的严重滥用，而不能替代适当的应用程序消息验证。
	OptionMaxRecvSize = "MAX-RCV-SIZE"

	//OptionReconnectTime是初始区间用于连接尝试。如果连接尝试没有成功，那么这个socket将等待很长时间再尝试。
	//一个可选的指数回溯可能会导致这个值的增长。看到OptionMaxReconnectTime更多细节。这是一个time.Duration。默认值是100毫秒的时间。
	//在开始任何dialers之前，必须先设置这个选项。
	OptionReconnectTime = "RECONNECT-TIME"

	//OptionMaxReconnectTime的最大值之间的连接尝试的时间
	//如果这个值为0，那么指数级的回退是禁用的，否则在尝试之间等待的值将加倍，直到达到这个极限
	//这个值是一个time.Duration。持续时间，初始值为0。在开始任何dialers之前，必须先设置这个选项
	OptionMaxReconnectTime = "MAX-RECONNECT-TIME"

	//OptionBestEffort使socket发送操作非阻塞。通常情况下(对于某些socket类型)
	//如果没有接收者，套接字将会阻塞，或者接收者无法与发送者保持同步。(多播sockets类型像Bus 或者 Star不能遵循)
	//如果此选项设置,没有阻止,而是默默地消息将被丢弃
	//值是一个布尔值，默认值为False。
	OptionBestEffort = "BEST-EFFORT"
)
