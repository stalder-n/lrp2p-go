# ATP - ARQ Transmission Protocol

## Introduction
ATP is a 0-RTT network protocol with reliability features and built-in asymmetric encryption. It is intended to be similar in features to TCP, while avoiding some of its drawbacks.

While ATP is a general purpose protocol, it is especially useful for developers dealing with peer-to-peer transfer problems, due to its inherent capability of sending and receiving simultaneously.

## Features
* Reliable data delivery
* Built-in asymmetric encryption
* Connectionless communication using UDP
* Native Go interface (ReadWriteCloser)

## Simple Example

### Client 1
```go
package main

import (
	"fmt"
	"github.com/nicosta1132/atp-go"
)

func main() {
    socket := atp.NewSocket("localhost", 3031, 3030)
    _, err := socket.Write([]byte("Hello World"))
    if err != nil {
		panic(err)
	}
}
```
### Client 2
```go
package main

import (
	"fmt"
	"github.com/nicosta1132/atp-go"
)

func main() {
    socket := atp.NewSocket("localhost", 3030, 3031)
    readBuffer := make([]byte, 32)
    _, err := socket.Read(readBuffer)
    fmt.Println(string(readBuffer))
    if err != nil {
        panic(err)
    }
}
```

## Specification

### Segment Header
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
