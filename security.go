package atp

import (
	"crypto/rand"
	"encoding/binary"
	"github.com/flynn/noise"
	"time"
)

type securityExtension struct {
	connector    connector
	strategy     handshakeStrategy
	handshake    *noise.HandshakeState
	encrypter    *noise.CipherState
	decrypter    *noise.CipherState
	key          *noise.DHKey
	peerKey      []byte
	writeNonce   uint64
	usedNonces   map[uint64]uint8
	errorChannel chan error
}

func (sec *securityExtension) ConnectTo(remoteHost string, remotePort int) {
	sec.connector.ConnectTo(remoteHost, remotePort)
}

func newSecurityExtension(connector connector, key *noise.DHKey, peerKey []byte, errors chan error) *securityExtension {
	newSec := &securityExtension{
		connector:    connector,
		writeNonce:   0,
		key:          key,
		peerKey:      peerKey,
		usedNonces:   make(map[uint64]uint8),
		errorChannel: errors,
	}
	return newSec
}

func (sec *securityExtension) addExtension(extension connector) {
	sec.connector = extension
}

func (sec *securityExtension) Close() error {
	return sec.connector.Close()
}

func (sec *securityExtension) Write(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	if sec.handshake == nil {
		payloadWritten := sec.initiateHandshake(buffer, timestamp)
		if payloadWritten {
			return success, len(buffer), nil
		}
	}
	if sec.encrypter == nil {
		return waitingForHandshake, 0, nil
	}
	encrypted := sec.encrypter.Cipher().Encrypt(nil, sec.writeNonce, nil, buffer)
	buf := make([]byte, 8+len(encrypted))
	copy(buf[8:], encrypted)
	binary.BigEndian.PutUint64(buf, sec.writeNonce)
	sec.writeNonce++
	return sec.connector.Write(buf, timestamp)
}

func (sec *securityExtension) Read(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	if sec.handshake == nil {
		payload := sec.acceptHandshake(timestamp)
		if payload != nil {
			copy(buffer, payload)
			return success, len(payload), nil
		}
	}
	if sec.decrypter == nil {
		return waitingForHandshake, 0, nil
	}
	encrypted := make([]byte, segmentMtu)
	statusCode, n, err := sec.connector.Read(encrypted, timestamp)
	if err != nil {
		sec.reportError(err)
		return statusCode, n, err
	}
	nonce := binary.BigEndian.Uint64(encrypted[:8])
	nonceStatus := sec.syncNonces(nonce)
	if nonceStatus != success {
		return nonceStatus, 0, nil
	}
	decryptedMsg, err := sec.decrypter.Cipher().Decrypt(nil, nonce, nil, encrypted[8:n])
	copy(buffer, decryptedMsg)
	return statusCode, len(decryptedMsg), err
}

func (sec *securityExtension) SetReadTimeout(t time.Duration) {
	sec.connector.SetReadTimeout(t)
}

func (sec *securityExtension) reportError(err error) {
	if err != nil {
		sec.errorChannel <- err
	}
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

func (sec *securityExtension) initiateHandshake(payload []byte, timestamp time.Time) (payloadWritten bool) {
	sec.determineHandshakeStrategy()
	sec.handshake = createHandshakeState(sec.key, sec.peerKey, sec.strategy.getPattern(), true)
	return sec.strategy.initiate(payload, timestamp)
}

func (sec *securityExtension) acceptHandshake(timestamp time.Time) []byte {
	sec.determineHandshakeStrategy()
	sec.handshake = createHandshakeState(sec.key, sec.peerKey, sec.strategy.getPattern(), false)
	return sec.strategy.accept(timestamp)
}

func (sec *securityExtension) writeHandshakeMessage(payload []byte, timestamp time.Time) (*noise.CipherState, *noise.CipherState) {
	msg, cs0, cs1, err := sec.handshake.WriteMessage(nil, payload)
	reportError(err)
	_, _, _ = sec.connector.Write(msg, timestamp)
	return cs0, cs1
}

// TODO set timeout for handshake
func (sec *securityExtension) readHandshakeMessage(timestamp time.Time) (statusCode, []byte, *noise.CipherState, *noise.CipherState) {
	readBuffer := make([]byte, segmentMtu)
	statusCode, n, _ := sec.connector.Read(readBuffer, timestamp)
	if statusCode == timeout {
		return timeout, nil, nil, nil
	}
	payload, cs0, cs1, err := sec.handshake.ReadMessage(nil, readBuffer[:n])
	reportError(err)
	return success, payload, cs0, cs1
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

// TODO correctly handle potential handshake timeout
type handshakeStrategy interface {
	initiate(payload []byte, timestamp time.Time) bool
	accept(timestamp time.Time) []byte
	getPattern() noise.HandshakePattern
}

type handshakeXXStrategy struct {
	sec *securityExtension
}

func (h *handshakeXXStrategy) initiate(payload []byte, timestamp time.Time) bool {
	h.sec.writeHandshakeMessage(nil, timestamp)
	h.sec.readHandshakeMessage(timestamp)
	h.sec.encrypter, h.sec.decrypter = h.sec.writeHandshakeMessage(nil, timestamp)
	return false
}

func (h *handshakeXXStrategy) accept(timestamp time.Time) []byte {
	state := timeout
	for ; state == timeout; {
		state, _, _, _ = h.sec.readHandshakeMessage(timestamp)
	}
	h.sec.writeHandshakeMessage(nil, timestamp)
	_, _, h.sec.decrypter, h.sec.encrypter = h.sec.readHandshakeMessage(timestamp)
	return nil
}

func (h *handshakeXXStrategy) getPattern() noise.HandshakePattern {
	return noise.HandshakeXX
}

type handshakeKKStrategy struct {
	sec *securityExtension
}

func (h *handshakeKKStrategy) initiate(payload []byte, timestamp time.Time) bool {
	h.sec.SetReadTimeout(1 * time.Second)
	defer h.sec.SetReadTimeout(0)

	code := fail
	for try := 0; code != success && try < 3; try++ {
		if try == 2 {
			h.sec.SetReadTimeout(3 * time.Second)
		}
		h.sec.writeHandshakeMessage(payload, timestamp)
		code, _, h.sec.encrypter, h.sec.decrypter = h.sec.readHandshakeMessage(timestamp)
	}
	if code != success {
		panic("failed to establish connection")
	}
	return true
}

func (h *handshakeKKStrategy) accept(timestamp time.Time) []byte {
	h.sec.SetReadTimeout(1 * time.Second)
	defer h.sec.SetReadTimeout(0)

	var payload []byte
	code := fail
	for try := 0; code != success && try < 3; try++ {
		if try == 2 {
			h.sec.SetReadTimeout(3 * time.Second)
		}
		code, payload, _, _ = h.sec.readHandshakeMessage(timestamp)

	}
	if code != success {
		panic("failed to establish connection")
	}
	h.sec.decrypter, h.sec.encrypter = h.sec.writeHandshakeMessage(nil, timestamp)

	return payload
}

func (h *handshakeKKStrategy) getPattern() noise.HandshakePattern {
	return noise.HandshakeKK
}
