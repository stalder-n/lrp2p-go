package main

type Message struct {
	Sequencenumber       uint32 // 32 Bit
	DataOffset           uint8  // 4 Bit and shows where the data begins (header length)
	Acknowledgmentnumber uint32 // 32 Bit and shows the next sequence number the sender of the ack is expecting to receive.
	Windowsize           uint16 // 16 Bit the number of data octets the sender of this message is willing to accept
	Checksum             uint16 // 16 Bit Checksum over the header
	Data                 []byte
}
