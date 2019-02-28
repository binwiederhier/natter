package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	serverFlag := flag.Bool("server", false, "Server mode")
	hubFlag := flag.String("hub", "", "Hub server")

	flag.Parse()

	if *serverFlag {
		if flag.NArg() < 1 {
			syntax()
		}

		listenAddr := flag.Arg(0)

		server := &server{}
		server.start(listenAddr)
	} else {
		if flag.NArg() < 1 || *hubFlag == "" {
			syntax()
		}

		spec := strings.Split(flag.Arg(0), ":")
		if len(spec) != 4 {
			syntax()
		}

		source := spec[0]
		sourcePort, err := strconv.Atoi(spec[1])
		if err != nil {
			syntax()
		}

		target := spec[2]
		targetPort, err := strconv.Atoi(spec[3])
		if err != nil {
			syntax()
		}

		client := &client{}
		client.start(*hubFlag, source, sourcePort, target, targetPort)
	}
}

func syntax() {
	fmt.Println("Syntax: natter -server :PORT")
	fmt.Println("        natter -hub HUBHOST LOCALUSER:LOCALPORT:TARGET:TARGETPORT")
	os.Exit(1)
}
