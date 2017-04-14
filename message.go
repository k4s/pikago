package pikago

import (
	"sync"
	"sync/atomic"
	"time"
)

//Message封装了来回交换的messages
//Header和Body字段的含义，以及分割发生的位置，将根据协议的不同而有所不同
//但是要注意,任何headers适用于传输层(包括TCP /ethernet headers和SP协议独立的headers长度)
type Message struct {
	Header []byte
	Body   []byte
	Port   Port
	bbuf   []byte
	hbuf   []byte
	bsize  int
	refcnt int32
	expire time.Time
	pool   *sync.Pool
}

type msgCacheInfo struct {
	maxbody int
	pool    *sync.Pool
}

func newMsg(sz int) *Message {
	m := &Message{}
	m.bbuf = make([]byte, 0, sz)
	m.hbuf = make([]byte, 0, 32)
	m.bsize = sz
	return m
}

// 按size切割多个Message的池
var messageCache = []msgCacheInfo{
	{
		maxbody: 64,
		pool: &sync.Pool{
			New: func() interface{} { return newMsg(64) },
		},
	}, {
		maxbody: 128,
		pool: &sync.Pool{
			New: func() interface{} { return newMsg(128) },
		},
	}, {
		maxbody: 256,
		pool: &sync.Pool{
			New: func() interface{} { return newMsg(256) },
		},
	}, {
		maxbody: 512,
		pool: &sync.Pool{
			New: func() interface{} { return newMsg(512) },
		},
	}, {
		maxbody: 1024,
		pool: &sync.Pool{
			New: func() interface{} { return newMsg(1024) },
		},
	}, {
		maxbody: 4096,
		pool: &sync.Pool{
			New: func() interface{} { return newMsg(4096) },
		},
	}, {
		maxbody: 8192,
		pool: &sync.Pool{
			New: func() interface{} { return newMsg(8192) },
		},
	}, {
		maxbody: 65536,
		pool: &sync.Pool{
			New: func() interface{} { return newMsg(65536) },
		},
	},
}

//Free 在message上减少引用计数，如果没有进一步的引用，则释放它的资源
//这样做可以让资源在不使用GC的情况下被回收。这对性能有相当大的好处。
func (m *Message) Free() {
	if v := atomic.AddInt32(&m.refcnt, -1); v > 0 {
		return
	}
	for i := range messageCache {
		if m.bsize == messageCache[i].maxbody {
			messageCache[i].pool.Put(m)
			return
		}
	}
}

//Dup创建了一个“duplicate”的message,它真正做的只是增加消息上的引用计数
// 请注意，由于底层message实际上是共享的，因此使用者必须注意不要修改消息。
//(未来修改这个API添加copy-on-write,但现在修改既不需要,也不支持。)
//应用程序应该不使用这个函数,它是用于协议,仅Transport和内部使用。
func (m *Message) Dup() *Message {
	atomic.AddInt32(&m.refcnt, 1)
	return m
}

//如果返回ture说明message已“expired”
//这是由transport实现用来丢弃在写队列中停留太长时间的消息，应该丢弃而不是通过transport传递
//这只是用于TX路径,是毫无意义的“expiration”Rw路径
func (m *Message) Expired() bool {
	if m.expire.IsZero() {
		return false
	}
	if m.expire.After(time.Now()) {
		return false
	}
	return true
}

//NewMessage是获得新Message的方式
//这就使用了一个“cache”，它极大地减少了垃圾收集器的负载
func NewMessage(sz int) *Message {
	var m *Message
	for i := range messageCache {
		if sz < messageCache[i].maxbody {
			m = messageCache[i].pool.Get().(*Message)
			break
		}
	}
	if m == nil {
		m = newMsg(sz)
	}

	m.refcnt = 1
	m.Body = m.bbuf
	m.Header = m.hbuf
	return m
}
