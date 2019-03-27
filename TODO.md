## L2:
- Mode "road warrior": Add remote tap to remote bridge, run dhclient on local interface

  natter "eth:pommes,dhcp,split,lroutes=auto,rroutes='10.0.1.2/22,10.0.2.0/24'"
  natter eth:pommes
     //= eth:pommes,warrior
     //= eth:pommes,lbridge=no,lroutes=auto,dhcp=yes,rbridge=auto,rroutes=
  natter eth:pommes,sites
  natter eth:pommes
    // alias for natter eth:pommes,dhcp=yes,rbridge=auto 
     
  natter tcp:8022:pommes:22
  natter tcp:8022:pommes:10.0.1.2:22
  
  - Flags:
    dhcp=yes [nodhcp|dhcp]
    routes=auto
    rbridge=auto|none|mybr..
    lbridge=none|mybr..
  
- Mode "site-to-site": Add both taps to bridges 

## Other things
- Kill remote command when connection is closed
- Shutdown connection when STDIN is closed
- Close connection when remote command/port closes
- Properly close goroutines/forwards
- Allow checking status of a forward via Forward struct
- Fix QUIC config
- make logging pretty
- Listen(), Forward(), ... should only return after a successful connection
