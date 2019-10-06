package main

type extension interface {
	openClose
	Write(buffer []byte)
	Read() []byte
	AddExtension(extension *extension)
}

type connectorAdapter struct {
	connector *Connector
}

func (adapter *connectorAdapter) Open() {
	(*adapter.connector).Open()
}

func (adapter *connectorAdapter) Close() {
	(*adapter.connector).Close()
}

func (adapter *connectorAdapter) Write(buffer []byte) {
	seg := createDefaultSegment(0, buffer)
	(*adapter.connector).Write(seg.buffer)
}

func (adapter *connectorAdapter) Read() []byte {
	buffer := (*adapter.connector).Read()
	seg := createSegment(buffer)
	return seg.data
}

func (adapter *connectorAdapter) AddExtension(extension *extension) {

}
