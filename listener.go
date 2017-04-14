package pikago

// Listener 用于transport和address的底层侦听器的接口
type Listener interface {

	//关闭listener，移除它在任何活跃的socket
	//重复操作将返回ErrClosed
	Close() error

	//Listen 开始监听新的connectons在address上面
	Listen() error

	// Address 返回Listener的字符串(完整URL)
	Address() string

	//SetOption设置一个选项在Listener
	//设置选项只能在Listen()调用之前完成
	SetOption(name string, value interface{}) error

	// GetOption 获取Listener的选项值
	GetOption(name string) (interface{}, error)
}
