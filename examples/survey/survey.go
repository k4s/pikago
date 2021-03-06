// survey implements a survey example.  server is a surveyor listening
// socket, and clients are dialing respondent sockets.
//
// To use:
//
//   $ go build .
//   $ url=tcp://127.0.0.1:40899
//   $ ./survey server $url server & server=$!
//   $ ./survey client $url client0 & client0=$!
//   $ ./survey client $url client1 & client1=$!
//   $ ./survey client $url client2 & client2=$!
//   $ sleep 5
//   $ kill $server $client0 $client1 $client2
//survey.exe server tcp://127.0.0.1:40899
//survey.exe client tcp://127.0.0.1:40899 client0
//survey.exe client tcp://127.0.0.1:40899 client1
//
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/k4s/pikago"
	"github.com/k4s/pikago/protocol/respondent"
	"github.com/k4s/pikago/protocol/surveyor"
	"github.com/k4s/pikago/transport/tcp"
)

func die(format string, v ...interface{}) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(format, v...))
	os.Exit(1)
}

func date() string {
	return time.Now().Format(time.ANSIC)
}

func server(url string) {
	var sock pikago.Socket
	var err error
	var msg []byte
	if sock, err = surveyor.NewSocket(); err != nil {
		die("can't get new surveyor socket: %s", err)
	}
	sock.AddTransport(tcp.NewTransport())
	if err = sock.Listen(url); err != nil {
		die("can't listen on surveyor socket: %s", err.Error())
	}
	err = sock.SetOption(pikago.OptionSurveyTime, time.Second/2)
	if err != nil {
		die("SetOption(): %s", err.Error())
	}
	for {
		time.Sleep(time.Second)
		fmt.Println("SERVER: SENDING DATE SURVEY REQUEST")
		if err = sock.Send([]byte("DATE")); err != nil {
			die("Failed sending survey: %s", err.Error())
		}
		for {
			if msg, err = sock.Recv(); err != nil {
				break
			}
			fmt.Printf("SERVER: RECEIVED \"%s\" SURVEY RESPONSE\n",
				string(msg))
		}
		fmt.Println("SERVER: SURVEY OVER")
	}
}

func client(url string, name string) {
	var sock pikago.Socket
	var err error
	var msg []byte

	if sock, err = respondent.NewSocket(); err != nil {
		die("can't get new respondent socket: %s", err.Error())
	}
	sock.AddTransport(tcp.NewTransport())
	if err = sock.Dial(url); err != nil {
		die("can't dial on respondent socket: %s", err.Error())
	}
	for {
		if msg, err = sock.Recv(); err != nil {
			die("Cannot recv: %s", err.Error())
		}
		fmt.Printf("CLIENT(%s): RECEIVED \"%s\" SURVEY REQUEST\n",
			name, string(msg))

		d := date()
		fmt.Printf("CLIENT(%s): SENDING DATE SURVEY RESPONSE\n", name)
		if err = sock.Send([]byte(d)); err != nil {
			die("Cannot send: %s", err.Error())
		}
	}
}

func main() {
	if len(os.Args) > 2 && os.Args[1] == "server" {
		server(os.Args[2])
		os.Exit(0)
	}
	if len(os.Args) > 3 && os.Args[1] == "client" {
		client(os.Args[2], os.Args[3])
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr, "Usage: survey server|client <URL> <ARG>\n")
	os.Exit(1)
}
