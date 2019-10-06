package depracated

import "strings"

type ReliableExtension struct {
	Extension    *Extension
	currentAcked int
}

func (ex *ReliableExtension) SendMessage(message string) {
	(*ex.Extension).SendMessage(message)
}
func (ex *ReliableExtension) ReadMessage() []byte {
	message := (*ex.Extension).ReadMessage()
	if !strings.Contains(string(message), "ACK") {
		(*ex.Extension).SendMessage("1: ACK " + string(message))
	}
	return message
}
func (ex *ReliableExtension) ReadStringMessage() string {
	return string(ex.ReadMessage())
}
func (ex *ReliableExtension) AddExtension(extension *Extension) {
	ex.Extension = extension
}
func (ex *ReliableExtension) Close() {
	(*ex.Extension).Close()
}
