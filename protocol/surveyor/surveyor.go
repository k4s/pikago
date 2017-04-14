// Package surveyor implements the SURVEYOR protocol. This sends messages
// out to RESPONDENT partners, and receives their responses.
package surveyor

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/k4s/pikago"
)

const defaultSurveyTime = time.Second

type surveyor struct {
	sock     pikago.ProtocolSocket
	peers    map[uint32]*surveyorP
	raw      bool
	nextID   uint32
	surveyID uint32
	duration time.Duration
	timeout  time.Time
	timer    *time.Timer
	w        pikago.Waiter
	init     sync.Once
	ttl      int

	sync.Mutex
}

type surveyorP struct {
	q  chan *pikago.Message
	ep pikago.Endpoint
	x  *surveyor
}

func (x *surveyor) Init(sock pikago.ProtocolSocket) {
	x.sock = sock
	x.peers = make(map[uint32]*surveyorP)
	x.sock.SetRecvError(pikago.ErrProtoState)
	x.timer = time.AfterFunc(x.duration,
		func() { x.sock.SetRecvError(pikago.ErrProtoState) })
	x.timer.Stop()
	x.w.Init()
	x.w.Add()
	go x.sender()
}

func (x *surveyor) Shutdown(expire time.Time) {

	x.w.WaitAbsTimeout(expire)
	x.Lock()
	peers := x.peers
	x.peers = make(map[uint32]*surveyorP)
	x.Unlock()

	for id, peer := range peers {
		delete(peers, id)
		pikago.DrainChannel(peer.q, expire)
		close(peer.q)
	}
}

func (x *surveyor) sender() {
	defer x.w.Done()
	cq := x.sock.CloseChannel()
	sq := x.sock.SendChannel()
	for {
		var m *pikago.Message
		select {
		case m = <-sq:
			if m == nil {
				sq = x.sock.SendChannel()
				continue
			}
		case <-cq:
			return
		}

		x.Lock()
		for _, pe := range x.peers {
			m := m.Dup()
			select {
			case pe.q <- m:
			default:
				m.Free()
			}
		}
		x.Unlock()
	}
}

// When sending, we should have the survey ID in the header.
func (peer *surveyorP) sender() {
	for {
		if m := <-peer.q; m == nil {
			break
		} else {
			if peer.ep.SendMsg(m) != nil {
				m.Free()
				return
			}
		}
	}
}

func (peer *surveyorP) receiver() {

	rq := peer.x.sock.RecvChannel()
	cq := peer.x.sock.CloseChannel()

	for {
		m := peer.ep.RecvMsg()
		if m == nil {
			return
		}
		if len(m.Body) < 4 {
			m.Free()
			continue
		}

		// Get survery ID -- this will be passed in the header up
		// to the application.  It should include that in the response.
		m.Header = append(m.Header, m.Body[:4]...)
		m.Body = m.Body[4:]

		select {
		case rq <- m:
		case <-cq:
			return
		}
	}
}

func (x *surveyor) AddEndpoint(ep pikago.Endpoint) {
	peer := &surveyorP{ep: ep, x: x, q: make(chan *pikago.Message, 1)}
	x.Lock()
	x.peers[ep.GetID()] = peer
	go peer.receiver()
	go peer.sender()
	x.Unlock()
}

func (x *surveyor) RemoveEndpoint(ep pikago.Endpoint) {
	id := ep.GetID()

	x.Lock()
	peer := x.peers[id]
	delete(x.peers, id)
	x.Unlock()

	if peer != nil {
		close(peer.q)
	}
}

func (*surveyor) Number() uint16 {
	return pikago.ProtoSurveyor
}

func (*surveyor) PeerNumber() uint16 {
	return pikago.ProtoRespondent
}

func (*surveyor) Name() string {
	return "surveyor"
}

func (*surveyor) PeerName() string {
	return "respondent"
}

func (x *surveyor) SendHook(m *pikago.Message) bool {

	if x.raw {
		return true
	}

	x.Lock()
	// fmt.Println("x.nextID", x.nextID)
	x.surveyID = x.nextID | 0x80000000
	// fmt.Println("x.surveyID", x.surveyID)
	x.nextID++
	x.sock.SetRecvError(nil)
	v := x.surveyID
	// fmt.Println("m.Header", m.Header)
	m.Header = append(m.Header,
		byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	// fmt.Println("m.Header", m.Header)
	// fmt.Println("byte(v>>24), byte(v>>16), byte(v>>8), byte(v)", byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	if x.duration > 0 {
		x.timer.Reset(x.duration)
	}
	x.Unlock()

	return true
}

func (x *surveyor) RecvHook(m *pikago.Message) bool {
	if x.raw {
		return true
	}

	x.Lock()
	defer x.Unlock()

	if len(m.Header) < 4 {
		return false
	}
	if binary.BigEndian.Uint32(m.Header) != x.surveyID {
		return false
	}
	m.Header = m.Header[4:]
	return true
}

func (x *surveyor) SetOption(name string, val interface{}) error {
	var ok bool
	switch name {
	case pikago.OptionRaw:
		if x.raw, ok = val.(bool); !ok {
			return pikago.ErrBadValue
		}
		if x.raw {
			x.timer.Stop()
			x.sock.SetRecvError(nil)
		} else {
			x.sock.SetRecvError(pikago.ErrProtoState)
		}
		return nil
	case pikago.OptionSurveyTime:
		x.Lock()
		x.duration, ok = val.(time.Duration)
		x.Unlock()
		if !ok {
			return pikago.ErrBadValue
		}
		return nil
	case pikago.OptionTTL:
		// We don't do anything with this, but support it for
		// symmetry with the respondent socket.
		if ttl, ok := val.(int); !ok {
			return pikago.ErrBadValue
		} else if ttl < 1 || ttl > 255 {
			return pikago.ErrBadValue
		} else {
			x.ttl = ttl
		}
		return nil
	default:
		return pikago.ErrBadOption
	}
}

func (x *surveyor) GetOption(name string) (interface{}, error) {
	switch name {
	case pikago.OptionRaw:
		return x.raw, nil
	case pikago.OptionSurveyTime:
		x.Lock()
		d := x.duration
		x.Unlock()
		return d, nil
	case pikago.OptionTTL:
		return x.ttl, nil
	default:
		return nil, pikago.ErrBadOption
	}
}

// NewSocket allocates a new Socket using the SURVEYOR protocol.
func NewSocket() (pikago.Socket, error) {
	return pikago.MakeSocket(&surveyor{duration: defaultSurveyTime}), nil
}
