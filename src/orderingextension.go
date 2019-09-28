package main

import (
	"sort"
	"strconv"
	"strings"
)

type OrderingExtension struct {
	Messages []string
	Counter  int
}

func (ex *OrderingExtension) AddMessage(message string) {
	ex.Counter += 1
	ex.Messages = append(ex.Messages, message)
	sort.Slice(ex.Messages, func(i int, j int) bool {
		return ex.Messages[i] < ex.Messages[j]
	})
}

func (ex *OrderingExtension) PopMessage() ([]byte, string) {
	var message string
	if len(ex.Messages) > 0 && strings.Split(ex.Messages[0], ":")[0] <= strconv.Itoa(ex.Counter) {
		message, ex.Messages = ex.Messages[0], ex.Messages[1:]
		return []byte(message), "ok"
	} else {
		return nil, "nok"
	}
}
