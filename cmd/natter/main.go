package main

import (
	"errors"
	"flag"
	"fmt"
	"heckel.io/natter"
	"os"
	"strings"
)

// TODO Close connection when remote command/port closes
// TODO properly close goroutines/forwards

func main() {
	serverFlag := flag.Bool("server", false, "Run in server mode")
	configFlag := flag.String("config", "", "Config file")
	nameFlag := flag.String("name", "", "Client name")
	brokerFlag := flag.String("broker", "", "Server address, e.g. example.com:9999")
	listenFlag := flag.Bool("listen", false, "Run client in listen mode")

	flag.Parse()

	if *serverFlag {
		runServer()
	} else {
		runClient(configFlag, nameFlag, brokerFlag, listenFlag)
	}
}

func runClient(configFlag *string, nameFlag *string, brokerFlag *string, listenFlag *bool) {
	config := loadConfig(configFlag, nameFlag, brokerFlag)
	client := createClient(config)

	// Process -listen flag
	if *listenFlag {
		err := client.ListenIncoming()
		if err != nil {
			fail(err)
		}
	}

	// Read forward specs and command
	var targetCommandStartIndex int
	var targetCommand []string
	var specs []string

	for i := 0; i < flag.NArg(); i++ {
		spec := strings.Split(flag.Arg(i), ":")
		if len(spec) != 3 && len(spec) != 4 {
			targetCommandStartIndex = i
			break
		}
	}

	if targetCommandStartIndex == 0 {
		specs = flag.Args()
		targetCommand = make([]string, 0)
	} else {
		specs = flag.Args()[:targetCommandStartIndex]
		targetCommand = flag.Args()[targetCommandStartIndex:]
	}

	if !*listenFlag && len(specs) == 0 {
		fail(errors.New("either specify the -listen flag or at least one forward spec"))
		syntax()
	}

	// Process forward specs
	for _, s := range specs {
		spec := strings.Split(s, ":")

		var (
			sourceAddr string
			target string
			targetForwardAddr string
		)

		if len(spec) == 3 {
			sourceAddr = spec[0]
			target = spec[1]

			if spec[2] == "" {
				targetForwardAddr = ""

				if len(targetCommand) == 0 {
					fail(errors.New("Invalid spec " + s + ", no command specified"))
				}
			} else {
				targetForwardAddr = ":" + spec[2]
			}
		} else if len(spec) == 4 {
			sourceAddr = spec[0]
			target = spec[1]
			targetForwardAddr = spec[2] + ":" + spec[3]
		}

		if sourceAddr != "" {
			sourceAddr = ":" + sourceAddr
		}

		_, err := client.Forward(sourceAddr, target, targetForwardAddr, targetCommand)
		if err != nil {
			fail(err)
		}
	}

	select { }
}

func parseSpec(spec []string) (sourceAddr string, target string, targetHost string, targetPort string) {
	if len(spec) == 3 {
		return spec[0], spec[1], "", spec[2]
	} else if len(spec) == 4 {
		return spec[0], spec[1], spec[2], spec[3]
	}

	return "", "", "", ""
}

func loadConfig(configFlag *string, nameFlag *string, brokerFlag *string) *natter.ClientConfig {
	var config *natter.ClientConfig
	var err error

	if *configFlag != "" {
		config, err = natter.LoadClientConfig(*configFlag)
		if err != nil {
			fmt.Println("Invalid config file:", err.Error())
			fmt.Println()
			syntax()
		}
	} else {
		config = &natter.ClientConfig{}
	}

	if *nameFlag != "" {
		config.ClientUser = *nameFlag
	}

	if *brokerFlag != "" {
		config.BrokerAddr = *brokerFlag
	}

	if config.ClientUser == "" {
		fmt.Println("Client name cannot be empty.")
		fmt.Println()
		syntax()
	}

	if config.BrokerAddr == "" {
		fmt.Println("Broker address cannot be empty.")
		fmt.Println()
		syntax()
	}

	return config
}

func createClient(config *natter.ClientConfig) *natter.Client {
	client, err := natter.NewClient(config)
	if err != nil {
		fail(err)
	}

	return client
}

func fail(err error) {
	fmt.Println(err.Error())
	os.Exit(1)
}

func runServer() {
	if flag.NArg() < 1 {
		syntax()
	}

	listenAddr := flag.Arg(0)

	server := natter.NewBroker()
	server.ListenAndServe(listenAddr)
}

func syntax() {
	fmt.Println("Syntax:")
	fmt.Println("  natter -server :PORT")
	fmt.Println("    Start the rendevous server on PORT for new client connections")
	fmt.Println()
	fmt.Println("  natter [-config CONFIG] [-name CLIENTNAME] [-broker BROKER] [-listen] [FORWARDSPEC ...] [COMMAND]")
	fmt.Println("    Start client side daemon to listen for incoming forwards")
	fmt.Println()
	fmt.Println("  Forward spec:")
	fmt.Println("    [LOCALPORT]:TARGET:[TARGETHOST:]TARGETPORT")
	fmt.Println("    Defines local input and remote input ports")
	fmt.Println()
	fmt.Println("    LOCALPORT:TARGET:TARGETPORT           - Forward local TCP port to target TCP port")
	fmt.Println("    LOCALPORT:TARGET:OTHERHOST:OTHERPORT  - Forward local TCP port to another host on target's network")
	fmt.Println("    LOCALPORT:TARGET: COMMAND             - Forward local TCP port to target command")
	fmt.Println("    :TARGET:TARGETPORT                    - Forward STDIN to target TCP port")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  natter -config alice.conf 8022:bob:22")
	fmt.Println("    Forward local TCP port 8022 to bob's TCP port 22")
	fmt.Println()
	fmt.Println("  natter -config alice.conf -listen 8022:bob:10.0.1.1:22")
	fmt.Println("    Forward local TCP port 8022 to TCP address 10.0.1.1:22 in bob's network,")
	fmt.Println("    and also listen for incoming forwards")
	fmt.Println()
	fmt.Println("  natter -name alice -broker example.com:1337 :bob: sh -c 'cat > file.txt'")
	fmt.Println("    Forward local STDIN to remote command")
	os.Exit(1)
}
