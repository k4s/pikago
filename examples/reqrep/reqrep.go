// reqprep implements a request/reply example.  node0 is a listening
// rep socket, and node1 is a dialing req socket.
//
// To use:
//
//   $ go build .
//   $ url=tcp://127.0.0.1:40899
//   $ ./reqrep node0 $url & node0=$! && sleep 1
//   $ ./reqrep node1 $url
//   $ kill $node0
//
//reqrep.exe node0 tcp://127.0.0.1:40899
//reqrep.exe node1 tcp://127.0.0.1:40899
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/k4s/pikago"
	"github.com/k4s/pikago/protocol/rep"
	"github.com/k4s/pikago/protocol/req"
	"github.com/k4s/pikago/transport/tcp"
)

func die(format string, v ...interface{}) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(format, v...))
	os.Exit(1)
}

func date() string {
	return time.Now().Format(time.ANSIC)
}

func node0(url string) {
	var sock pikago.Socket
	var err error
	var msg []byte
	if sock, err = rep.NewSocket(); err != nil {
		die("can't get new rep socket: %s", err)
	}
	sock.AddTransport(tcp.NewTransport())
	if err = sock.Listen(url); err != nil {
		die("can't listen on rep socket: %s", err.Error())
	}
	for {
		// Could also use sock.RecvMsg to get header
		msg, err = sock.Recv()
		if string(msg) == "DATE" { // no need to terminate
			fmt.Println("NODE0: RECEIVED DATE REQUEST")
			d := date()
			fmt.Printf("NODE0: SENDING DATE %s\n", d)
			err = sock.Send([]byte(d))
			if err != nil {
				die("can't send reply: %s", err.Error())
			}
		}
	}
}

func node1(url string) {
	var sock pikago.Socket
	var err error
	var msg []byte

	if sock, err = req.NewSocket(); err != nil {
		die("can't get new req socket: %s", err.Error())
	}
	sock.AddTransport(tcp.NewTransport())
	if err = sock.Dial(url); err != nil {
		die("can't dial on req socket: %s", err.Error())
	}
	fmt.Printf("NODE1: SENDING DATE REQUEST %s\n", "DATE")
	for {
		if err = sock.Send([]byte("DATE")); err != nil {
			die("can't send message on push socket: %s", err.Error())
		}
		if msg, err = sock.Recv(); err != nil {
			die("can't receive date: %s", err.Error())
		}
		fmt.Printf("NODE1: RECEIVED DATE %s\n", string(msg))
		time.Sleep(time.Second)
	}

	sock.Close()
}

func main() {
	if len(os.Args) > 2 && os.Args[1] == "node0" {
		node0(os.Args[2])
		os.Exit(0)
	}
	if len(os.Args) > 2 && os.Args[1] == "node1" {
		node1(os.Args[2])
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr, "Usage: reqrep node0|node1 <URL>\n")
	os.Exit(1)
}
