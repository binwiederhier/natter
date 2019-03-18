package main

import (
	"heckel.io/natter"
	"time"
)

func main() {
/*	c := make(chan struct{})
	wg := sync.WaitGroup{}

	f := func() {
		for {
			select {
			case <- c:
				println("bla")
				wg.Done()
				return
			default:
				println("no bla")
				time.Sleep(1 * time.Second)
			}
		}
	}

	wg.Add(2)
	go f()
	go f()

	time.Sleep(5 * time.Second)

	println("closing")
	close(c)

	wg.Wait()
	println("done")
	*/

	broker := natter.NewBroker()
	go broker.ListenAndServe(":5000")

	alice, _ := natter.NewClient(&natter.ClientConfig{
		ClientUser: "alice",
		BrokerAddr: "localhost:5000",
	})
	alice.Listen()

	time.Sleep(2 * time.Second)

	println("donedone")
}
