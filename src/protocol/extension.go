package protocol

type extension interface {
	Connector
	addExtension(extension extension)
}

type extensionDelegator struct {
	extension extension
}

func (connection *extensionDelegator) Open() {
	connection.extension.Open()
}

func (connection *extensionDelegator) Close() {
	connection.extension.Close()
}

func (connection *extensionDelegator) Write(buffer []byte) {
	connection.extension.Write(buffer)
}

func (connection *extensionDelegator) Read() []byte {
	return connection.extension.Read()
}

func (connection *extensionDelegator) addExtension(extension extension) {
	connection.extension = extension
}

type connectorAdapter struct {
	connector Connector
}

func (adapter *connectorAdapter) Open() {
	adapter.connector.Open()
}

func (adapter *connectorAdapter) Close() {
	adapter.connector.Close()
}

func (adapter *connectorAdapter) Write(buffer []byte) {
	seg := createDefaultSegment(0, buffer)
	adapter.connector.Write(seg.buffer)
}

func (adapter *connectorAdapter) Read() []byte {
	buffer := adapter.connector.Read()
	seg := createSegment(buffer)
	return seg.data
}

func (adapter *connectorAdapter) addExtension(extension extension) {

}
