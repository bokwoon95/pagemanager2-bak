// Package encrypthash provides both encryption and hashing.
package encrypthash

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/nacl/secretbox"
)

type Box struct {
	key        []byte
	getKeys    func() (keys [][]byte, err error)
	processKey func(keyIn []byte) (keyOut []byte, err error)
}

var (
	ErrNoKey             = errors.New("no key found")
	ErrCiphertextInvalid = errors.New("ciphertext invalid")
	ErrHashInvalid       = errors.New("hash invalid")
	ErrHashedMsgInvalid  = errors.New("hashed message invalid")
)

func NewStaticKey(key []byte) (Box, error) {
	if len(key) == 0 {
		return Box{}, fmt.Errorf("key length cannot be 0")
	}
	return Box{key: key}, nil
}

func NewRotatingKeys(getKeys func() (keys [][]byte, err error), processKey func(keyIn []byte) (keyOut []byte, err error)) (Box, error) {
	if getKeys == nil {
		return Box{}, fmt.Errorf("getKeys function cannot be nil")
	}
	return Box{getKeys: getKeys, processKey: processKey}, nil
}

func (box Box) Encrypt(plaintext []byte) (ciphertext []byte, err error) {
	const nonceSize = 24
	var key []byte
	if box.getKeys != nil {
		var keys [][]byte
		keys, err = box.getKeys()
		if err != nil {
			return nil, err
		}
		if len(keys) == 0 {
			return nil, ErrNoKey
		}
		key = keys[0]
	} else {
		if len(box.key) == 0 {
			return nil, ErrNoKey
		}
		key = box.key
	}
	if box.processKey != nil {
		key, err = box.processKey(key)
		if err != nil {
			return nil, err
		}
	}
	hashedKey := blake2b.Sum512(key)
	var hashKeyUpper [32]byte
	copy(hashKeyUpper[:], hashedKey[:32])
	var nonce [nonceSize]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	ciphertext = secretbox.Seal(nonce[:], plaintext, &nonce, &hashKeyUpper)
	return ciphertext, nil
}

func (box Box) Decrypt(ciphertext []byte) (plaintext []byte, err error) {
	const nonceSize = 24
	var keys [][]byte
	if box.getKeys != nil {
		keys, err = box.getKeys()
		if err != nil {
			return nil, err
		}
		if len(keys) == 0 {
			return nil, ErrNoKey
		}
	} else {
		if len(box.key) == 0 {
			return nil, ErrNoKey
		}
		keys = [][]byte{box.key}
	}
	for _, key := range keys {
		if box.processKey != nil {
			key, err = box.processKey(key)
			if err != nil {
				return nil, err
			}
		}
		hashedKey := blake2b.Sum512(key)
		var hashedKeyUpper [32]byte
		copy(hashedKeyUpper[:], hashedKey[:32])
		var nonce [nonceSize]byte
		copy(nonce[:], ciphertext[:nonceSize])
		plaintext, ok := secretbox.Open(nil, ciphertext[nonceSize:], &nonce, &hashedKeyUpper)
		if !ok {
			continue
		}
		return plaintext, nil
	}
	return nil, ErrCiphertextInvalid
}

func (box Box) Hash(msg []byte) (hash []byte, err error) {
	var key []byte
	if box.getKeys != nil {
		var keys [][]byte
		keys, err = box.getKeys()
		if err != nil {
			return nil, err
		}
		if len(keys) == 0 {
			return nil, ErrNoKey
		}
		key = keys[0]
	} else {
		if len(box.key) == 0 {
			return nil, ErrNoKey
		}
		key = box.key
	}
	if box.processKey != nil {
		key, err = box.processKey(key)
		if err != nil {
			return nil, err
		}
	}
	hashedKey := blake2b.Sum512([]byte(key))
	hashedKeyLower := hashedKey[32:]
	h, _ := blake2b.New512(hashedKeyLower)
	h.Reset()
	h.Write([]byte(msg))
	sum := h.Sum(nil)
	return sum, nil
}

func (box Box) HashAll(msg []byte) (hashes [][]byte, err error) {
	var keys [][]byte
	if box.getKeys != nil {
		keys, err = box.getKeys()
		if err != nil {
			return nil, err
		}
		if len(keys) == 0 {
			return nil, ErrNoKey
		}
	} else {
		if len(box.key) == 0 {
			return nil, ErrNoKey
		}
		keys = [][]byte{box.key}
	}
	for _, key := range keys {
		if box.processKey != nil {
			key, err = box.processKey(key)
			if err != nil {
				return hashes, err
			}
		}
		hashedKey := blake2b.Sum512([]byte(key))
		hashedKeyLower := hashedKey[32:]
		h, _ := blake2b.New512(hashedKeyLower)
		h.Reset()
		h.Write([]byte(msg))
		hash := h.Sum(nil)
		hashes = append(hashes, hash)
	}
	return hashes, nil
}

func (box Box) VerifyHash(msg []byte, hash []byte) error {
	var err error
	var keys [][]byte
	if box.getKeys != nil {
		keys, err = box.getKeys()
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			return ErrNoKey
		}
	} else {
		if len(box.key) == 0 {
			return ErrNoKey
		}
		keys = [][]byte{box.key}
	}
	for _, key := range keys {
		if box.processKey != nil {
			key, err = box.processKey(key)
			if err != nil {
				return err
			}
		}
		hashedKey := blake2b.Sum512([]byte(key))
		hashedKeyLower := hashedKey[32:]
		h, _ := blake2b.New512(hashedKeyLower)
		h.Reset()
		h.Write([]byte(msg))
		computedHash := h.Sum(nil)
		if subtle.ConstantTimeCompare(computedHash, hash) == 1 {
			return nil
		}
	}
	return ErrHashInvalid
}

func (box Box) Base64Encrypt(plaintext []byte) (b64Ciphertext []byte, err error) {
	ciphertext, err := box.Encrypt(plaintext)
	if err != nil {
		return nil, err
	}
	b64Ciphertext = make([]byte, base64.RawURLEncoding.EncodedLen(len(ciphertext)))
	base64.RawURLEncoding.Encode(b64Ciphertext, ciphertext)
	return b64Ciphertext, nil
}

func (box Box) Base64Decrypt(b64Ciphertext []byte) (plaintext []byte, err error) {
	ciphertext := make([]byte, base64.RawURLEncoding.DecodedLen(len(b64Ciphertext)))
	n, err := base64.RawURLEncoding.Decode(ciphertext, b64Ciphertext)
	if err != nil {
		return nil, err
	}
	ciphertext = ciphertext[:n]
	plaintext, err = box.Decrypt(ciphertext)
	return plaintext, err
}

func (box Box) Base64Hash(msg []byte) (b64HashedMsg []byte, err error) {
	hash, err := box.Hash(msg)
	if err != nil {
		return nil, err
	}
	b64Msg := make([]byte, base64.RawURLEncoding.EncodedLen(len(msg)))
	base64.RawURLEncoding.Encode(b64Msg, msg)
	b64Hash := make([]byte, base64.RawURLEncoding.EncodedLen(len(hash)))
	base64.RawURLEncoding.Encode(b64Hash, hash)
	b64HashedMsg = append(b64HashedMsg, b64Msg...)
	b64HashedMsg = append(b64HashedMsg, '~')
	b64HashedMsg = append(b64HashedMsg, b64Hash...)
	return b64HashedMsg, nil
}

func (box Box) Base64VerifyHash(b64HashedMsg []byte) (msg []byte, err error) {
	dotIndex := -1
	for i, c := range b64HashedMsg {
		if c == '~' {
			dotIndex = i
			break
		}
	}
	if dotIndex < 0 {
		return nil, ErrHashedMsgInvalid
	}
	b64Msg := b64HashedMsg[:dotIndex]
	msg = make([]byte, base64.RawURLEncoding.DecodedLen(len(b64Msg)))
	n, err := base64.RawURLEncoding.Decode(msg, b64Msg)
	if err != nil {
		return nil, err
	}
	msg = msg[:n]
	b64Hash := b64HashedMsg[dotIndex+1:]
	hash := make([]byte, base64.RawURLEncoding.DecodedLen(len(b64Hash)))
	n, err = base64.RawURLEncoding.Decode(hash, b64Hash)
	if err != nil {
		return nil, err
	}
	err = box.VerifyHash(msg, hash)
	if err != nil {
		return nil, err
	}
	return msg, nil
}
