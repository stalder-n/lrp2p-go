# LRP2P - Lightweight Reliable Peer 2 Peer

![go test](https://github.com/stalder-n/lrp2p/workflows/go%20test/badge.svg)

## Introduction

LRP2P is a peer to peer network protocol with reliability features and built-in asymmetric encryption. It's intended to
be similar in features to TCP, while avoiding some of its drawbacks.

## Features

* Reliable data delivery
* Peer-to-Peer; every socket can maintain unlimited peers
* Built-in asymmetric encryption
* Connectionless communication using UDP
* Native Go interface (ReadWriteCloser)

## Simple Example

### Peer 1

```go
package main

import (
	p2p "github.com/stalder-n/lrp2p"
)

func main() {
	socket := p2p.SocketListen("", 3030)
	conn := socket.ConnectTo("<Peer 2 ip address>", 3030)
	_, err := conn.Write([]byte("Hello World"))
	if err != nil {
		panic(err)
	}
}
```

### Peer 2

```go
package main

import (
	"fmt"
	p2p "github.com/stalder-n/lrp2p"
)

func main() {
	socket := p2p.SocketListen("", 3030)
	conn := socket.Accept()
	readBuffer := make([]byte, 32)
	n, err := conn.Read(readBuffer)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(readBuffer[:n]))
}
```

## Multiplexing example

### Peer 1

```go
package main

import (
	"fmt"
	p2p "github.com/stalder-n/lrp2p"
)

func main() {
	socket := p2p.SocketListen("", 3030)
	conn1 := socket.Accept()
	conn2 := socket.Accept()

	// read from first connection
	readBuffer := make([]byte, 64)
	n, err := conn1.Read(readBuffer)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(readBuffer[:n]))

	// read from first connection
	n, err = conn2.Read(readBuffer)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(readBuffer[:n]))
}
```

### Peer 2

```go
package main

import (
	p2p "github.com/stalder-n/lrp2p"
)

func main() {
	socket := p2p.SocketListen("", 3030)
	conn := socket.ConnectTo("<Peer 1 ip address>", 3030)
	_, err := conn.Write([]byte("Hello from Peer 2"))
	if err != nil {
		panic(err)
	}
}
```

### Peer 3

```go
package main

import (
	p2p "github.com/stalder-n/lrp2p"
)

func main() {
	socket := p2p.SocketListen("", 3030)
	conn := socket.ConnectTo("<Peer 1 ip address>", 3030)
	_, err := conn.Write([]byte("Hello from Peer 3"))
	if err != nil {
		panic(err)
	}
}
```

## Specification

### Data Segment Header

| Data Offset (1B) | Flags (1B) | Sequence Number (4B) | 
| ---------------- | ---------- | -------------------- |

```
Data Offset:
    Byte designating the start of payload data

Flags:
    Single byte for flag bits

Sequence Number:
    32-bit sequence number designating a segments order
```

### ACK Segment

| Data Offset (1B) | Flags (1B) | Sequence Number (4B) | Window Size (3B) |
| ---------------- | ---------- | -------------------- | ---------------- |

```
Data Offset:
    Byte designating the start of payload data

Flags:
    Single byte for flag bits

Sequence Number:
    32-bit sequence number designating a segments order. For ACKS, this is always the last in-order number

Window Size:
    24-bit number telling the sender how large its congestion window may grow
```

### Window Size of the SND and RCV buffer

The buffer size needs to be at least as large as the [BDP](https://en.wikipedia.org/wiki/Bandwidth-delay_product). TCP
can increase the window size of to 1'073'725'440 and is measured in bytes. LRP2P measures in packets, and a packet size
of 1400 is assumed.

To allow a 100Gibt/s over a sattelite link with 600ms RTT one has:

```
100'000'000'000 x 0.6 / 8 = 7.5 GByte
```

which means that 7.5GB can be in transit. The number of packets is ~5.3mio assuming packet size of 1400. Thus, 23bit
would fit, and therefore we chose 24bit, 3 bytes to store the window size.
