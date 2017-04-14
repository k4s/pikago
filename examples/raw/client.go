package main

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/k4s/pikago"
	"github.com/k4s/pikago/protocol/req"
	"github.com/k4s/pikago/transport/tcp"
)

// synchronize our output messaging so we don't overlap
var lock sync.Mutex

func clientWorker(url string, id int) {
	var sock pikago.Socket
	var m *pikago.Message
	var err error

	if sock, err = req.NewSocket(); err != nil {
		die("can't get new req socket: %s", err.Error())
	}

	// Leave this in Cooked mode!

	sock.AddTransport(tcp.NewTransport())
	if err = sock.Dial(url); err != nil {
		die("can't dial on req socket: %s", err.Error())
	}

	// send an empty messsage
	m = pikago.NewMessage(1)
	if err = sock.SendMsg(m); err != nil {
		die("can't send request: %s", err.Error())
	}

	if m, err = sock.RecvMsg(); err != nil {
		die("can't recv reply: %s", err.Error())
	}
	sock.Close()

	if len(m.Body) != 4 {
		die("bad response len: %d", len(m.Body))
	}

	worker := binary.BigEndian.Uint32(m.Body[0:])

	lock.Lock()
	fmt.Printf("Client: %4d   Server: %4d\n", id, worker)
	lock.Unlock()
}

func client(url string, nworkers int) {

	var wg sync.WaitGroup
	for i := 0; i < nworkers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			clientWorker(url, i)
		}(i)
	}
	wg.Wait()

}
