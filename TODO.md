
- Close connection when remote command/port closes
- Properly close goroutines/forwards
- Allow checking status of a forward via Forward struct
- Make protobuf structs internal
- Fix QUIC config
- make logging pretty
- Listen(), Forward(), ... should only return after a successful connection
