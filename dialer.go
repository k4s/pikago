package pikago

// Dialer 是用于transport和address的底层拨号器的接口。
type Dialer interface {
	//关闭dialer，移除它在任何活跃的socket
	//重复操作将返回ErrClosed
	Close() error

	//Dial 开始connecting在address上面，如果连接失败将重新开始
	Dial() error

	// Address 返回Listener的字符串(完整URL)
	Address() string

	//SetOption设置一个选项在Listener
	//设置选项只能在Listen()调用之前完成
	SetOption(name string, value interface{}) error

	// GetOption 获取Listener的选项值
	GetOption(name string) (interface{}, error)
}
