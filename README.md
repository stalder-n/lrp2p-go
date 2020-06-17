# ATP - ARQ Transmission Protocol
![go test](https://github.com/nicosta1132/atp-go/workflows/go%20test/badge.svg)

## Introduction
ATP is a 0-RTT network protocol with reliability features and built-in asymmetric encryption. It is intended to be similar in features to TCP, while avoiding some of its drawbacks.

While ATP is a general purpose protocol, it is especially useful for developers dealing with peer-to-peer transfer problems, due to its inherent capability of sending and receiving simultaneously.

## Features
* Reliable data delivery
* Built-in asymmetric encryption
* Connectionless communication using UDP
* Native Go interface (ReadWriteCloser)

## Simple Example

### Peer 1
```go
package main

import (
    "fmt"
    "github.com/nicosta1132/atp-go"
)

func main() {
    socket := atp.SocketListen("127.0.0.1", 3030)
    socket.ConnectTo("localhost", 3031)
    _, err := socket.Write([]byte("Hello World"))
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
    "github.com/nicosta1132/atp-go"
)

func main() {
    socket := atp.SocketListen("127.0.0.1", 3031)
    socket.ConnectTo("localhost", 3030)
    readBuffer := make([]byte, 32)
    n, err := socket.Read(readBuffer)
    fmt.Println(string(readBuffer[:n]))
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
| Data Offset (1B) | Flags (1B) | Sequence Number (4B) | Window Size (4B) |
| ---------------- | ---------- | -------------------- | ---------------- |

```
Data Offset:
    Byte designating the start of payload data

Flags:
    Single byte for flag bits

Sequence Number:
    32-bit sequence number designating a segments order. For ACKS, this is always the last in-order number

Window Size:
    32-bit number telling the sender how large its congestion window may grow
```