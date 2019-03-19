# natter: A peer-to-peer TCP port forwarding library using NAT traversal with QUIC

**natter** is a peer-to-peer TCP port forwarding library and command line tool. It connects two clients across the Internet, 
even if they are behind a NAT via [UDP hole punching](https://en.wikipedia.org/wiki/UDP_hole_punching) 
([NAT traversal](https://en.wikipedia.org/wiki/NAT_traversal)) as per [RFC 5128](https://tools.ietf.org/html/rfc5128#section-3.3.1).
Connections are brokered via a rendevous server ("broker"), and tunneled via the [QUIC](https://en.wikipedia.org/wiki/QUIC) protocol.

The command line utility `natter` implements the broker and the client. The library is natively written in Go, but 
provides a C library (and can be used in C/C++).  

## Building

```
make cmd
make lib
```

## Examples

We provide a few code examples for Go, C and C++ in the [example](example/) directory. Please note that for obvious
reasons, all examples operate on localhost, but they could work across multiple systems.

You can also run examples via `make`.
 
### With the CLI

Let's assume we have 3 machines, an Internet facing broker and two clients behind different NATs alice and bob.
Alice 

First, start the broker on address 1.2.3.4:10000:
```
broker> build/cmd/natter -server 1.2.3.4:10000
```

Then start client Bob, and listen for incoming connections:
```
bob> natter -name bob -broker 1.2.3.4:10000 -listen
```
   
And finally start client Alice, and forward local TCP connections on 8022 to bob's TCP port 22 (SSH). After that, 
Alice can connect to Bob's SSH server by connecting to localhost:8022:
```
alice> natter -name alice -broker 1.2.3.4:10000 8022:bob:22
alice> ssh -p 8022 root@localhost
```

### With the Go library

Here's the same example on localhost:

```go
package main

import (
    "heckel.io/natter"
)

func main() {
	broker, _ := natter.NewBroker(&natter.Config{BrokerAddr: ":10000"})
	go broker.ListenAndServe()

	bob, _ := natter.NewClient(&natter.Config{ClientUser: "bob", BrokerAddr: "localhost:10000"})
	bob.Listen()

	alice, _ := natter.NewClient(&natter.Config{ClientUser: "alice", BrokerAddr: "localhost:10000"})
	alice.Forward(":8022", "bob", ":22", nil)

	select {}
}
``` 

## Contributing

We are always happy to welcome new contributors!

## Author

Philipp C. Heckel
