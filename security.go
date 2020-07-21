package atp

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"github.com/flynn/noise"
	"sync"
)

// TODO execute handshake when establishing connection
//    	- connection initiator is also handshake initiator
//    	- start read procedure on connection establishment
type encryption struct {
	handshake        *noise.HandshakeState
	encrypter        *noise.CipherState
	decrypter        *noise.CipherState
	errorChannel     chan error
	writeNonce       uint64
	usedNonces       map[uint64]uint8
	messageNotifier  *sync.Cond
	handshakeMessage []byte
}

func newEncryption(errors chan error) *encryption {
	newSec := &encryption{
		errorChannel:    errors,
		usedNonces:      make(map[uint64]uint8),
		messageNotifier: sync.NewCond(&sync.Mutex{}),
	}
	return newSec
}

func (enc *encryption) Encrypt(in, out []byte) (int, error) {
	if enc.handshake == nil || enc.encrypter == nil {
		return 0, fmt.Errorf("connection not secured")
	}
	encrypted := enc.encrypter.Cipher().Encrypt(nil, enc.writeNonce, nil, in)
	copy(out[8:], encrypted)
	binary.BigEndian.PutUint64(out, enc.writeNonce)
	enc.writeNonce++
	return 8 + len(encrypted), nil
}

func (enc *encryption) Decrypt(in, out []byte) (int, error) {
	if enc.handshake == nil || enc.decrypter == nil {
		return 0, fmt.Errorf("connection not secured")
	}
	nonce := binary.BigEndian.Uint64(in[:8])
	valid := enc.syncNonces(nonce)
	if !valid {
		return 0, fmt.Errorf("nonce reuse detected")
	}
	decryptedMsg, err := enc.decrypter.Cipher().Decrypt(nil, nonce, nil, in[8:])
	if err != nil {
		return 0, fmt.Errorf("decryption failed")
	}
	copy(out, decryptedMsg)
	return len(decryptedMsg), nil
}

// Checks if the received nonce has been used before and returns an appropriate
// status code
func (enc *encryption) syncNonces(nonce uint64) bool {
	if _, ok := enc.usedNonces[nonce]; ok {
		return false
	}
	enc.usedNonces[nonce] = 1
	return true
}

func (enc *encryption) reportError(err error) {
	if err != nil {
		enc.errorChannel <- err
	}
}

func (enc *encryption) InitiateHandshake(c *Conn) connState {
	enc.handshake = createHandshakeState(noise.HandshakeXX, true)
	enc.writeHandshakeMessage(c)
	enc.readHandshakeMessage()
	enc.encrypter, enc.decrypter = enc.writeHandshakeMessage(c)
	return connected
}

func (enc *encryption) AcceptHandshake(c *Conn) connState {
	enc.handshake = createHandshakeState(noise.HandshakeXX, false)
	enc.readHandshakeMessage()
	enc.writeHandshakeMessage(c)
	enc.decrypter, enc.encrypter = enc.readHandshakeMessage()
	return connected
}

func (enc *encryption) writeHandshakeMessage(c *Conn) (*noise.CipherState, *noise.CipherState) {
	msg, cs0, cs1, err := enc.handshake.WriteMessage(nil, nil)
	reportError(err)
	buffer := make([]byte, len(msg)+8)
	binary.BigEndian.PutUint64(buffer, c.getConnId())
	copy(buffer[8:], msg)
	_, _, _ = c.writeToEndpoint(buffer)
	return cs0, cs1
}

func (enc *encryption) readHandshakeMessage() (*noise.CipherState, *noise.CipherState) {
	enc.messageNotifier.L.Lock()
	for enc.handshakeMessage == nil {
		enc.messageNotifier.Wait()
	}
	_, cs0, cs1, err := enc.handshake.ReadMessage(nil, enc.handshakeMessage)
	enc.messageNotifier.L.Unlock()
	reportError(err)
	enc.handshakeMessage = nil
	return cs0, cs1
}

func createHandshakeState(handshakePattern noise.HandshakePattern, isInitiator bool) *noise.HandshakeState {
	suite := noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2b)
	key, _ := suite.GenerateKeypair(rand.Reader)
	handshake, _ := noise.NewHandshakeState(noise.Config{
		CipherSuite:   suite,
		Random:        rand.Reader,
		Pattern:       handshakePattern,
		Initiator:     isInitiator,
		StaticKeypair: key,
	})
	return handshake
}
