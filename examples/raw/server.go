package main

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/k4s/pikago"
	"github.com/k4s/pikago/protocol/rep"
	"github.com/k4s/pikago/transport/tcp"
)

// Our protocol is simple.  Request packet is empty.  The reply
// is the replier's ID and the time it delayed responding for (us).

func serverWorker(sock pikago.Socket, id int) {
	var err error

	delay := rand.Intn(int(time.Second))

	for {
		var m *pikago.Message

		if m, err = sock.RecvMsg(); err != nil {
			return
		}

		m.Body = make([]byte, 4)

		time.Sleep(time.Duration(delay))

		binary.BigEndian.PutUint32(m.Body[0:], uint32(id))

		if err = sock.SendMsg(m); err != nil {
			return
		}
	}
}

func server(url string, nworkers int) {
	var sock pikago.Socket
	var err error
	var wg sync.WaitGroup

	rand.Seed(time.Now().UnixNano())

	if sock, err = rep.NewSocket(); err != nil {
		die("can't get new rep socket: %s", err)
	}
	if err = sock.SetOption(pikago.OptionRaw, true); err != nil {
		die("can't set raw mode: %s", err)
	}
	sock.AddTransport(tcp.NewTransport())
	if err = sock.Listen(url); err != nil {
		die("can't listen on rep socket: %s", err.Error())
	}
	wg.Add(nworkers)
	fmt.Printf("Starting %d workers\n", nworkers)
	for id := 0; id < nworkers; id++ {
		go func(id int) {
			defer wg.Done()
			serverWorker(sock, id)
		}(id)
	}
	wg.Wait()
}
