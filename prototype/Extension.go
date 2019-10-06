package main

type Extension interface {
	SendMessage(message string)
	ReadMessage() []byte
	ReadStringMessage() string
	AddExtension(extension *Extension)
	Close()
}
