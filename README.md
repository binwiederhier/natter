# natter: A peer-to-peer TCP port forwarding library using NAT traversal with QUIC

**natter** is a peer-to-peer TCP port forwarding library and command line tool. It connects two clients across the Internet, 
even if they are behind a NAT via [UDP hole punching](https://en.wikipedia.org/wiki/UDP_hole_punching) 
([NAT traversal](https://en.wikipedia.org/wiki/NAT_traversal)) as per [RFC 5128](https://tools.ietf.org/html/rfc5128#section-3.3.1).
Connections are brokered (but not relayed!) via a rendevous server ("broker"), and tunneled via the [QUIC](https://en.wikipedia.org/wiki/QUIC) protocol.

The command line utility `natter` implements the broker and the client. The library is natively written in Go, but 
provides a C library (and can be used in C/C++).  

## Project Status

While the functionality is very close to what I want the library to be, the project and library API is a 
**work in progress**, mostly because this is my first Go project.

## Building

You'll need [Go 1.12+](https://golang.org/) and a [protobuf compiler](https://developers.google.com/protocol-buffers/). 
Pretty much everything can be compiled with `make`:

```
Build:
  make all   - Build all deliverables
  make cmd   - Build the natter CLI tool & Go library
  make lib   - Build the natter C/C++ library
  make clean - Clean build folder

Examples:
  example_echo_[_run]      - Build/run echo client/server example
  example_simple_go[_run]  - Build/run simple Go example
  example_simple_c[_run]   - Build/run simple C example
  example_simple_cpp[_run] - Build/run simple C++ example
```

## Examples

We provide a few code examples for Go, C and C++ in the [example](example/) directory. Please note that for obvious
reasons, all examples operate on localhost, but they could work across multiple systems.

You can also run examples via `make`.
 
### TCP port forwarding using the natter CLI

Let's assume we have 3 machines, an Internet facing broker and two clients behind different NATs alice and bob.

First, start the broker on port 10000 (let's assume it listens on IP 1.2.3.4):
```
broker> natter -broker :10000
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

## STDIN-to-remote-command forwarding using the natter CLI

This is a fun example. It forwards the output of a local command to the input of a remote command, again, assuming 
that your broker is listening on 1.2.3.4:10000 and your remote client is Bob, like above:

```
alice> cat /etc/natter/natter.conf
ClientId alice
BrokerAddr 1.2.3.4:10000

alice> cat /dev/zero | natter :bob: sh -c 'cat > zeros'
```

### Using the Go library

Here's the same example on two clients and a broker on localhost. To run it, first ensure that you are using Go modules by initializing a module via `go mod init main`. Then create `nattertest.go`:

```go
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
	alice.Forward(":8022", "bob", ":22", nil)

	select {}
}
``` 

And finally run it via `go run nattertest.go`. This will get all the dependencies first and then run program:

```
$ go run nattertest.go
go: finding heckel.io/natter v0.0.6
go: downloading heckel.io/natter v0.0.6
go: extracting heckel.io/natter v0.0.6
....
2019/09/22 10:09:07 Waiting for connections
2019/09/22 10:09:07 Connecting to broker at 127.0.0.1:10000
2019/09/22 10:09:07 -> [CheckinRequest] Source:"bob" 
2019/09/22 10:09:07 <- [CheckinRequest] Source:"bob" 
2019/09/22 10:09:07 Client bob with address 127.0.0.1:17473 connected
...
```

## Contributing

There is lots [TODO](TODO.md). Feel free to help out via PRs of by opening issues.

## Author

Philipp C. Heckel
