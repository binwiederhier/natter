package main

import (
	"flag"
	"fmt"
	"heckel.io/natter"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano()) // TODO urgh!

	serverCommand := flag.NewFlagSet("server", flag.ExitOnError)
	daemonCommand := flag.NewFlagSet("daemon", flag.ExitOnError)
	forwardCommand := flag.NewFlagSet("forward", flag.ExitOnError)

	daemonHub := daemonCommand.String("hub", "", "Hub server")
	daemonName := daemonCommand.String("name", "", "Client name")
	forwardHub := forwardCommand.String("hub", "", "Hub server")
	forwardName := forwardCommand.String("name", "", "Client name")

	if len(os.Args) < 2 {
		syntax()
	}

	command := os.Args[1]

	switch command {
	case "daemon":
		if err := daemonCommand.Parse(os.Args[2:]); err != nil {
			syntax()
		}

		if *daemonHub == "" || *daemonName == "" {
			syntax()
		}

		daemon := natter.NewDaemon()
		daemon.Start(*daemonHub, *daemonName)
	case "forward":
		if err := forwardCommand.Parse(os.Args[2:]); err != nil {
			syntax()
		}

		if forwardCommand.NArg() < 1 || *forwardHub == "" || *forwardName == "" {
			syntax()
		}

		spec := strings.Split(forwardCommand.Arg(0), ":")
		if len(spec) != 4 {
			syntax()
		}

		sourcePort, err := strconv.Atoi(spec[0])
		if err != nil {
			syntax()
		}

		target := spec[1]
		targetForwardHost := spec[2]
		targetForwardPort, err := strconv.Atoi(spec[3])
		if err != nil {
			syntax()
		}
		targetForwardAddr := fmt.Sprintf("%s:%d", targetForwardHost, targetForwardPort)

		forwarder := natter.NewForwarder()
		forwarder.Start(*forwardHub, *forwardName, sourcePort, target, targetForwardAddr)
	case "server":
		if err := serverCommand.Parse(os.Args[2:]); err != nil {
			syntax()
		}

		if serverCommand.NArg() < 1 {
			syntax()
		}

		listenAddr := serverCommand.Arg(0)

		server := natter.NewServer()
		server.Start(listenAddr)
	default:
		syntax()
	}
}

func syntax() {
	fmt.Println("Syntax:")
	fmt.Println()
	fmt.Println("natter server :PORT")
	fmt.Println("  Start the rendevous server on PORT for new client connections")
	fmt.Println()
	fmt.Println("natter daemon -hub HUBHOST -name LOCALUSER")
	fmt.Println("  Start client side daemon to listen for incoming forwards")
	fmt.Println()
	fmt.Println("natter forward -hub HUBHOST -name LOCALUSER LOCALPORT:TARGET:[TARGETHOST]:TARGETPORT")
	fmt.Println("  Forwarding TCP traffic from local port LOCALPORT to target machine TARGETPORT")
	os.Exit(1)
}
