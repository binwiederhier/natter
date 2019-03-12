package main

import (
	"flag"
	"fmt"
	"heckel.io/natter"
	"os"
	"strings"
)

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
		err := client.Listen()
		if err != nil {
			fail(err)
		}
	}

	// Process forward specs
	for i := 0; i < flag.NArg(); i++ {
		spec := strings.Split(flag.Arg(i), ":")

		if len(spec) == 3 {
			sourceAddr := ":" + spec[0]
			target := spec[1]
			targetForwardAddr := ":" + spec[2]

			_, err := client.Forward(sourceAddr, target, targetForwardAddr)
			if err != nil {
				fail(err)
			}
		} else if len(spec) == 4 {
			sourceAddr := ":" + spec[0]
			target := spec[1]
			targetForwardAddr := spec[2] + ":" + spec[3]

			_, err := client.Forward(sourceAddr, target, targetForwardAddr)
			if err != nil {
				fail(err)
			}
		} else {
			syntax()
		}
	}

	select { }
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

	server := natter.NewServer()
	server.Start(listenAddr)
}

func syntax() {
	fmt.Println("Syntax:")
	fmt.Println("  natter -server :PORT")
	fmt.Println("    Start the rendevous server on PORT for new client connections")
	fmt.Println()
	fmt.Println("  natter [-config CONFIG] [-name CLIENTNAME] [-broker BROKER] [-listen] FORWARDSPEC ...")
	fmt.Println("    Start client side daemon to listen for incoming forwards")
	fmt.Println()
	fmt.Println("  Forward spec:")
	fmt.Println("    LOCALPORT:TARGET:TARGETPORT")
	fmt.Println("    LOCALPORT:TARGET:TARGETHOST:TARGETPORT")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  natter -config alice.conf 8022:bob:22")
	fmt.Println("    Forward local TCP port 8022 to bob's TCP port 22")
	fmt.Println()
	fmt.Println("  natter -config alice.conf -listen 8022:bob:10.0.1.1:22")
	fmt.Println("    Forward local TCP port 8022 to TCP address 10.0.1.1:22 in bob's network,")
	fmt.Println("    and also listen for incoming forwards")
	os.Exit(1)
}
