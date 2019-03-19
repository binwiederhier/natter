package main

import (
	"errors"
	"flag"
	"fmt"
	"heckel.io/natter"
	"log"
	"os"
	"strings"
)

const (
	defaultConfigFile = "/etc/natter/natter.conf"
)

func main() {
	configFlag := flag.String("config", "", "Config file, defaults to /etc/natter/natter.conf")
	brokerFlag := flag.String("broker", "", "Broker address and port")
	clientFlag := flag.String("client", "", "Client identifier (client only)")
	listenFlag := flag.Bool("listen", false, "Listen for incoming forwards (client only)")

	flag.Parse()

	config := loadConfig(configFlag, clientFlag, brokerFlag)

	if config.ClientId != "" {
		runClient(config, listenFlag)
	} else {
		runBroker(config)
	}
}

func runClient(config *natter.Config, listenFlag *bool) {
	if config.BrokerAddr == "" {
		fmt.Println("Broker address cannot be empty.")
		fmt.Println()
		syntax()
	}

	log.Println("Starting natter in client mode, client ID is " + config.ClientId)

	client, err := natter.NewClient(config)
	if err != nil {
		fail(err)
	}

	// Process -listen flag
	if *listenFlag {
		err := client.Listen()
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

func runBroker(config *natter.Config) {
	if flag.NArg() > 0 {
		config.BrokerAddr = flag.Arg(0)
	}

	if config.BrokerAddr == "" {
		fmt.Println("Broker address cannot be empty.")
		fmt.Println()
		syntax()
	}

	log.Println("Starting natter in broker mode, listening on " + config.BrokerAddr)

	broker, err := natter.NewBroker(config)
	if err != nil {
		fail(err)
	}

	if err := broker.ListenAndServe(); err != nil {
		fail(err)
	}
}

func loadConfig(configFlag *string, clientFlag *string, brokerFlag *string) *natter.Config {
	var config *natter.Config
	var err error

	if *configFlag != "" {
		config, err = natter.LoadConfig(*configFlag)
		if err != nil {
			fmt.Println(err.Error())
			fmt.Println()
			syntax()
		}
	} else if _, err := os.Stat(defaultConfigFile); err == nil {
		config, err = natter.LoadConfig(defaultConfigFile)
		if err != nil {
			fmt.Println(err.Error())
		}
	} else {
		config = &natter.Config{}
	}

	if *clientFlag != "" {
		config.ClientId = *clientFlag
	}

	if *brokerFlag != "" {
		config.BrokerAddr = *brokerFlag
	}

	return config
}

func syntax() {
	fmt.Println("Syntax:")
	fmt.Println("  natter -broker :PORT")
	fmt.Println("    Start the broker / rendevous server on PORT for new client connections")
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

func fail(err error) {
	fmt.Println(err.Error())
	os.Exit(2)
}
