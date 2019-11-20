package go_protocol

import (
	"crypto/rand"
	"encoding/binary"
	"github.com/flynn/noise"
	"log"
)

type securityExtension struct {
	connector  Connector
	strategy   handshakeStrategy
	handshake  *noise.HandshakeState
	encrypter  *noise.CipherState
	decrypter  *noise.CipherState
	key        *noise.DHKey
	peerKey    []byte
	writeNonce uint64
	usedNonces map[uint64]uint8
}

func newSecurityExtension(connector Connector, key *noise.DHKey, peerKey []byte) *securityExtension {
	newSec := &securityExtension{
		connector:  connector,
		writeNonce: 0,
		key:        key,
		peerKey:    peerKey,
		usedNonces: make(map[uint64]uint8),
	}
	return newSec
}

func (sec *securityExtension) addExtension(extension Connector) {
	sec.connector = extension
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
	if sec.encrypter == nil {
		return waitingForHandshake, 0, nil
	}

	encrypted := sec.encrypter.Cipher().Encrypt(nil, sec.writeNonce, nil, buffer)
	buf := make([]byte, 8+len(encrypted))
	copy(buf[8:], encrypted)
	binary.BigEndian.PutUint64(buf, sec.writeNonce)
	sec.writeNonce++
	return sec.connector.Write(buf)
}

func (sec *securityExtension) Read(buffer []byte) (statusCode, int, error) {
	if sec.handshake == nil {
		sec.acceptHandshake()
	}
	if sec.decrypter == nil {
		return waitingForHandshake, 0, nil
	}
	encrypted := make([]byte, len(buffer))
	statusCode, n, err := sec.connector.Read(encrypted)
	nonce := binary.BigEndian.Uint64(encrypted[:8])
	nonceStatus := sec.syncNonces(nonce)
	if nonceStatus != success {
		return nonceStatus, 0, nil
	}
	decryptedMsg, err := sec.decrypter.Cipher().Decrypt(nil, nonce, nil, encrypted[8:n])
	copy(buffer, decryptedMsg)
	return statusCode, len(decryptedMsg), err
}

// Checks if the received nonce has been used before and returns an appropriate
// status code
func (sec *securityExtension) syncNonces(nonce uint64) statusCode {
	if _, ok := sec.usedNonces[nonce]; ok {
		return invalidNonce
	}
	sec.usedNonces[nonce] = 1
	return success
}

func (sec *securityExtension) initiateHandshake() {
	sec.determineHandshakeStrategy()
	sec.handshake = createHandshakeState(sec.key, sec.peerKey, sec.strategy.getPattern(), true)
	sec.strategy.initiate()
}

func (sec *securityExtension) acceptHandshake() {
	sec.determineHandshakeStrategy()
	sec.handshake = createHandshakeState(sec.key, sec.peerKey, sec.strategy.getPattern(), false)
	sec.strategy.accept()
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

func (sec *securityExtension) determineHandshakeStrategy() {
	if sec.peerKey != nil {
		sec.strategy = &handshakeKKStrategy{sec}
	} else {
		sec.strategy = &handshakeXXStrategy{sec}
	}
}

func createHandshakeState(keyRef *noise.DHKey, peerKey []byte, handshakePattern noise.HandshakePattern, isInitiator bool) *noise.HandshakeState {
	suite := noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2b)
	var key noise.DHKey
	if keyRef == nil {
		key, _ = suite.GenerateKeypair(rand.Reader)
	} else {
		key = *keyRef
	}
	handshake, _ := noise.NewHandshakeState(noise.Config{
		CipherSuite:   suite,
		Random:        rand.Reader,
		Pattern:       handshakePattern,
		Initiator:     isInitiator,
		StaticKeypair: key,
		PeerStatic:    peerKey,
	})
	return handshake
}

func reportError(err error) {
	if err != nil {
		log.Println(err)
	}
}

type handshakeStrategy interface {
	initiate()
	accept()
	getPattern() noise.HandshakePattern
}

type handshakeXXStrategy struct {
	sec *securityExtension
}

func (h *handshakeXXStrategy) initiate() {
	h.sec.writeHandshakeMessage()
	h.sec.readHandshakeMessage()
	h.sec.encrypter, h.sec.decrypter = h.sec.writeHandshakeMessage()
}

func (h *handshakeXXStrategy) accept() {
	h.sec.readHandshakeMessage()
	h.sec.writeHandshakeMessage()
	h.sec.decrypter, h.sec.encrypter = h.sec.readHandshakeMessage()
}

func (h *handshakeXXStrategy) getPattern() noise.HandshakePattern {
	return noise.HandshakeXX
}

type handshakeKKStrategy struct {
	sec *securityExtension
}

func (h *handshakeKKStrategy) initiate() {
	h.sec.writeHandshakeMessage()
	h.sec.encrypter, h.sec.decrypter = h.sec.readHandshakeMessage()
}

func (h *handshakeKKStrategy) accept() {
	h.sec.readHandshakeMessage()
	h.sec.decrypter, h.sec.encrypter = h.sec.writeHandshakeMessage()
}

func (h *handshakeKKStrategy) getPattern() noise.HandshakePattern {
	return noise.HandshakeKK
}
