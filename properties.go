package pikago

//下面是在Port上公开的属性。

const (
	//PropLocalAddr表达一个本地地址。
	//对于dialers,这是本地的(通常是随机的)地址。对于listeners，它通常是服务地址。
	//值是一个net.Addr
	PropLocalAddr = "LOCAL-ADDR"

	//PropRemoteAddr表示远程地址。
	//对于dialers，这是服务地址。对于listeners，它是远端dialer的地址
	//值是一个net.Addr
	PropRemoteAddr = "REMOTE-ADDR"

	// PropTLSConnState is used to supply TLS connection details. The
	// value is a tls.ConnectionState.  It is only valid when TLS is used.
	//PropTLSConnState用于供应TLS连接细节
	//值是一个tls.ConnectionState，只有在使用TLS时才有效
	PropTLSConnState = "TLS-STATE"

	//PropHTTPRequest传达一个* http.Request
	//这个属性只存在于websocket连接中。
	PropHTTPRequest = "HTTP-REQUEST"
)
