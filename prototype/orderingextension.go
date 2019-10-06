package main

import (
	"sort"
	"strconv"
	"strings"
)

type OrderingExtension struct {
	Messages  []string
	Counter   int
	Extension *Extension
}

func (ex *OrderingExtension) addMessage(message string) {
	ex.Counter += 1
	ex.Messages = append(ex.Messages, message)
	sort.Slice(ex.Messages, func(i int, j int) bool {
		return ex.Messages[i] < ex.Messages[j]
	})
}

func (ex *OrderingExtension) ReadMessage() []byte {
	var message string
	if len(ex.Messages) > 0 && strings.Split(ex.Messages[0], ":")[0] <= strconv.Itoa(ex.Counter) {
		message, ex.Messages = ex.Messages[0], ex.Messages[1:]
		return []byte(message)
	} else {
		ex.addMessage((*ex.Extension).ReadStringMessage())
		return ex.ReadMessage()
	}
}

func (ex *OrderingExtension) ReadStringMessage() string {
	return string(ex.ReadMessage())
}

func (ex *OrderingExtension) SendMessage(message string) {
	(*ex.Extension).SendMessage(message)
}

func (channel *OrderingExtension) AddExtension(extension *Extension) {
	channel.Extension = extension
}

func (ex *OrderingExtension) Close() {
	(*ex.Extension).Close()
}
