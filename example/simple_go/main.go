package main

import (
	"heckel.io/natter"
)

func main() {
	broker, _ := natter.NewBroker(&natter.Config{BrokerAddr: ":10000"})
	go broker.ListenAndServe()

	bob, _ := natter.NewClient(&natter.Config{ClientId: "bob", BrokerAddr: "localhost:10000"})
	bob.Listen()

	alice, _ := natter.NewClient(&natter.Config{ClientId: "alice", BrokerAddr: "localhost:10000"})
	alice.Forward(":9000", "bob", ":22", nil)

	select {}
}
