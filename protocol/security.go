package protocol

import (
	"crypto/rand"
	"encoding/binary"
	"github.com/deckarep/golang-set"
	"github.com/flynn/noise"
	"log"
)

type securityExtension struct {
	connector     Connector
	handshake     *noise.HandshakeState
	encrypter     *noise.CipherState
	decrypter     *noise.CipherState
	writeNonce    uint64
	readNonce     uint64
	missingNonces mapset.Set
}

func (arq *securityExtension) addExtension(extension Connector) {
	arq.connector = extension
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

// Ensures the received nonce is valid, i.e. a nonce that hasn't been used before.
// To account for packet loss or wrong order, all skipped nonces are saved in a set
// that is updated accordingly as nonces are used
func (sec *securityExtension) syncNonces(nonce uint64) statusCode {
	if nonce > sec.readNonce {
		for i := sec.readNonce + 1; i < nonce; i++ {
			sec.missingNonces.Add(i)
		}
		sec.readNonce = nonce
		return success
	} else {
		it := sec.missingNonces.Iterator()
		for n := range it.C {
			if n.(uint64) == nonce {
				it.Stop()
				sec.missingNonces.Remove(n)
				sec.readNonce = nonce
				return success
			}
		}
		it.Stop()
	}
	return invalidNonce
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
	sec.missingNonces = mapset.NewSet(uint64(0))
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
	sec.missingNonces = mapset.NewSet(uint64(0))
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
		log.Println(err)
	}
}
