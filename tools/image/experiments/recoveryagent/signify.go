package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// signify file format (OpenBSD signify, as read in signify.c of the portable
// version): line 1 is "untrusted comment: ...", line 2 is base64 of
//
//	public key:  "Ed" + 8 bytes key number + 32 bytes Ed25519 public key
//	signature:   "Ed" + 8 bytes key number + 64 bytes Ed25519 signature
//
// The signature is a plain Ed25519 signature over the message bytes.

const (
	keyNumLen = 8
	pkAlg     = "Ed"
)

// PublicKey is one signify public key.
type PublicKey struct {
	KeyNum [keyNumLen]byte
	Key    ed25519.PublicKey
}

// ID is the key number in hex (what signify calls the key number).
func (k PublicKey) ID() string { return fmt.Sprintf("%x", k.KeyNum[:]) }

func decodeBlob(file []byte, want int) ([]byte, error) {
	lines := strings.Split(strings.TrimRight(string(file), "\n"), "\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[0], "untrusted comment: ") {
		return nil, errors.New("not a signify file: no comment line")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[1]))
	if err != nil {
		return nil, fmt.Errorf("not a signify file: %v", err)
	}
	if len(raw) != want || !bytes.HasPrefix(raw, []byte(pkAlg)) {
		return nil, errors.New("not a signify Ed25519 file of the expected size")
	}
	return raw, nil
}

// ParsePublicKey reads the text of a signify .pub file.
func ParsePublicKey(file []byte) (PublicKey, error) {
	raw, err := decodeBlob(file, 2+keyNumLen+ed25519.PublicKeySize)
	if err != nil {
		return PublicKey{}, err
	}
	var k PublicKey
	copy(k.KeyNum[:], raw[2:2+keyNumLen])
	k.Key = ed25519.PublicKey(append([]byte(nil), raw[2+keyNumLen:]...))
	return k, nil
}

// Signature is a parsed signify signature.
type Signature struct {
	KeyNum [keyNumLen]byte
	Sig    []byte
}

// ParseSignatureBlob reads the second line of a signify .sig file (base64 only).
func ParseSignatureBlob(b64 string) (Signature, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return Signature{}, err
	}
	if len(raw) != 2+keyNumLen+ed25519.SignatureSize || !bytes.HasPrefix(raw, []byte(pkAlg)) {
		return Signature{}, errors.New("signature has the wrong size or algorithm")
	}
	var s Signature
	copy(s.KeyNum[:], raw[2:2+keyNumLen])
	s.Sig = raw[2+keyNumLen:]
	return s, nil
}

// Verify checks sig over msg with k. The key numbers must match, as signify requires.
func (k PublicKey) Verify(msg []byte, s Signature) bool {
	return s.KeyNum == k.KeyNum && ed25519.Verify(k.Key, msg, s.Sig)
}
