// pair implements a pair example.  node0 is a listening
// pair socket, and node1 is a dialing pair socket.
//
// To use:
//
//   $ go build .
//   $ url=tcp://127.0.0.1:40899
//   $ ./pair node0 $url & node0=$!
//   $ ./pair node1 $url & node1=$!
//   $ sleep 3
//   $ kill $node0 $node1
// pair.exe node0 tcp://127.0.0.1:40899
// pair.exe node1 tcp://127.0.0.1:40899
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/k4s/pikago"
	"github.com/k4s/pikago/protocol/pair"
	"github.com/k4s/pikago/transport/inproc"
	"github.com/k4s/pikago/transport/tcp"
)

func die(format string, v ...interface{}) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(format, v...))
	os.Exit(1)
}

func sendName(sock pikago.Socket, name string) {
	fmt.Printf("%s: SENDING \"%s\"\n", name, name)
	if err := sock.Send([]byte(name)); err != nil {
		die("failed sending: %s", err)
	}
}

func recvName(sock pikago.Socket, name string) {
	var msg []byte
	var err error
	if msg, err = sock.Recv(); err == nil {
		fmt.Printf("%s: RECEIVED: \"%s\"\n", name, string(msg))
	}
}

func sendRecv(sock pikago.Socket, name string) {
	for {
		sock.SetOption(pikago.OptionRecvDeadline, 100*time.Millisecond)
		recvName(sock, name)
		time.Sleep(time.Second)
		sendName(sock, name)
	}
}

func node0(url string) {
	var sock pikago.Socket
	var err error
	if sock, err = pair.NewSocket(); err != nil {
		die("can't get new pair socket: %s", err)
	}
	sock.AddTransport(inproc.NewTransport())
	sock.AddTransport(tcp.NewTransport())
	if err = sock.Listen(url); err != nil {
		die("can't listen on pair socket: %s", err.Error())
	}
	sendRecv(sock, "node0")
}

func node1(url string) {
	var sock pikago.Socket
	var err error

	if sock, err = pair.NewSocket(); err != nil {
		die("can't get new pair socket: %s", err.Error())
	}
	sock.AddTransport(inproc.NewTransport())
	sock.AddTransport(tcp.NewTransport())
	if err = sock.Dial(url); err != nil {
		die("can't dial on pair socket: %s", err.Error())
	}
	sendRecv(sock, "node1")
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
	fmt.Fprintf(os.Stderr, "Usage: pair node0|node1 <URL>\n")
	os.Exit(1)
}
