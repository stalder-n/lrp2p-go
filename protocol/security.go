package protocol

import (
	"crypto/rand"
	"github.com/flynn/noise"
)

type securityExtension struct {
	connector Connector
	handshake *noise.HandshakeState
	encrypter *noise.CipherState
	decrypter *noise.CipherState
}

func (sec *securityExtension) Open() error {
	return sec.connector.Open()
}

func (sec *securityExtension) Close() error {
	return sec.connector.Close()
}

func (sec *securityExtension) Write(buffer []byte) (statusCode, int, error) {
	if sec.handshake == nil {
		sec.initiateHandshake()
	}
	return sec.connector.Write(sec.encrypter.Encrypt(nil, nil, buffer))
}

func (sec *securityExtension) Read(buffer []byte) (statusCode, int, error) {
	if sec.handshake == nil {
		sec.acceptHandshake()
	}

	encryptedMsg := make([]byte, len(buffer))
	statusCode, n, err := sec.connector.Read(encryptedMsg)
	decryptedMsg, err := sec.decrypter.Decrypt(nil, nil, encryptedMsg[:n])
	copy(buffer, decryptedMsg)
	return statusCode, len(decryptedMsg), err
}

func (sec *securityExtension) initiateHandshake() {
	suite := noise.NewCipherSuite(noise.DH25519, noise.CipherAESGCM, noise.HashBLAKE2b)
	key, _ := suite.GenerateKeypair(rand.Reader)
	sec.handshake, _ = noise.NewHandshakeState(noise.Config{
		CipherSuite:   suite,
		Random:        rand.Reader,
		Pattern:       noise.HandshakeXX,
		Initiator:     true,
		StaticKeypair: key,
	})
	sec.writeHandshakeMessage()
	sec.readHandshakeMessage()
	sec.encrypter, sec.decrypter = sec.writeHandshakeMessage()
}

func (sec *securityExtension) acceptHandshake() {
	suite := noise.NewCipherSuite(noise.DH25519, noise.CipherAESGCM, noise.HashBLAKE2b)
	key, _ := suite.GenerateKeypair(rand.Reader)
	sec.handshake, _ = noise.NewHandshakeState(noise.Config{
		CipherSuite:   suite,
		Random:        rand.Reader,
		Pattern:       noise.HandshakeXX,
		Initiator:     false,
		StaticKeypair: key,
	})
	sec.readHandshakeMessage()
	sec.writeHandshakeMessage()
	sec.decrypter, sec.encrypter = sec.readHandshakeMessage()
}

func (sec *securityExtension) writeHandshakeMessage() (*noise.CipherState, *noise.CipherState) {
	msg, cs0, cs1, err := sec.handshake.WriteMessage(nil, nil)
	reportError(err)
	_, _, _ = sec.connector.Write(msg)
	return cs0, cs1
}

func (sec *securityExtension) readHandshakeMessage() (*noise.CipherState, *noise.CipherState) {
	readBuffer := make([]byte, 128)
	_, n, _ := sec.connector.Read(readBuffer)
	_, cs0, cs1, err := sec.handshake.ReadMessage(nil, readBuffer[:n])
	reportError(err)
	return cs0, cs1
}

func reportError(err error) {
	if err != nil {
		panic(err.Error())
	}
}
