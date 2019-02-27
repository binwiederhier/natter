package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type ChatRequest struct {
	Action   string
	Username string
	Message  string
}

func main() {
	server := flag.Bool("server", false, "Server mode")
	hub := flag.String("hub", "", "Hub server")

	flag.Parse()

	if *server {
		if flag.NArg() < 1 {
			syntax()
		}

		listenAddr := flag.Arg(0)
		println(listenAddr)
		startServer(listenAddr)
	} else {
		if flag.NArg() < 1 || *hub == "" {
			syntax()
		}

		spec := strings.Split(flag.Arg(0), ":")
		if len(spec) != 4 {
			syntax()
		}

		localUser := spec[0]
		localPort, err := strconv.Atoi(spec[1])
		if err != nil {
			syntax()
		}

		target := spec[2]
		targetPort, err := strconv.Atoi(spec[3])
		if err != nil {
			syntax()
		}

		startClient(*hub, localUser, localPort, target, targetPort)
	}
}

func syntax() {
	fmt.Println("Syntax: natter -server :PORT")
	fmt.Println("        natter -hub HUBHOST LOCALUSER:LOCALPORT:TARGET:TARGETPORT")
	os.Exit(1)
}
