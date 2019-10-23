package protocol

type extension interface {
	Connector
	addExtension(extension extension)
}

type extensionDelegator struct {
	extension extension
}

func (connection *extensionDelegator) Open() error {
	return connection.extension.Open()
}

func (connection *extensionDelegator) Close() error {
	return connection.extension.Close()
}

func (connection *extensionDelegator) Write(buffer []byte) (int, error) {
	return connection.extension.Write(buffer)
}

func (connection *extensionDelegator) Read(buffer []byte) (int, error) {
	return connection.extension.Read(buffer)
}

func (connection *extensionDelegator) addExtension(extension extension) {
	connection.extension = extension
}

type connectorAdapter struct {
	connector Connector
}

func (adapter *connectorAdapter) Open() error {
	return adapter.connector.Open()
}

func (adapter *connectorAdapter) Close() error {
	return adapter.connector.Close()
}

func (adapter *connectorAdapter) Write(buffer []byte) (int, error) {
	return adapter.connector.Write(buffer)
}

func (adapter *connectorAdapter) Read(buffer []byte) (int, error) {
	return adapter.connector.Read(buffer)
}

func (adapter *connectorAdapter) addExtension(extension extension) {
}
