// Package encrypthash provides both encryption and hashing.
package encrypthash

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/nacl/secretbox"
)

const nonceSize = 24

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
	hashedKey := blake2b.Sum512(key)
	hashedKeyLower := hashedKey[32:]
	h, _ := blake2b.New512(hashedKeyLower)
	h.Reset()
	h.Write(msg)
	hash = h.Sum(nil)
	return hash, nil
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
		hashedKey := blake2b.Sum512(key)
		hashedKeyLower := hashedKey[32:]
		h, _ := blake2b.New512(hashedKeyLower)
		h.Reset()
		h.Write(msg)
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
		hashedKey := blake2b.Sum512(key)
		hashedKeyLower := hashedKey[32:]
		h, _ := blake2b.New512(hashedKeyLower)
		h.Reset()
		h.Write(msg)
		computedHash := h.Sum(nil)
		if subtle.ConstantTimeCompare(computedHash, hash) == 1 {
			return nil
		}
	}
	return ErrHashInvalid
}

func base64Encode(src []byte) []byte {
	buf := make([]byte, base64.RawURLEncoding.EncodedLen(len(src)))
	base64.RawURLEncoding.Encode(buf, src)
	return buf
}

func base64Decode(src []byte) ([]byte, error) {
	dbuf := make([]byte, base64.RawURLEncoding.DecodedLen(len(src)))
	n, err := base64.RawURLEncoding.Decode(dbuf, src)
	return dbuf[:n], err
}

func (box Box) Base64Encrypt(plaintext []byte) (b64Ciphertext []byte, err error) {
	ciphertext, err := box.Encrypt(plaintext)
	if err != nil {
		return nil, err
	}
	return base64Encode(ciphertext), nil
}

func (box Box) Base64Decrypt(b64Ciphertext []byte) (plaintext []byte, err error) {
	ciphertext, err := base64Decode(b64Ciphertext)
	if err != nil {
		return nil, err
	}
	plaintext, err = box.Decrypt(ciphertext)
	return plaintext, err
}

func (box Box) Base64Hash(msg []byte) (b64HashedMsg []byte, err error) {
	hash, err := box.Hash(msg)
	if err != nil {
		return nil, err
	}
	b64HashedMsg = append(b64HashedMsg, base64Encode(msg)...)
	b64HashedMsg = append(b64HashedMsg, '~')
	b64HashedMsg = append(b64HashedMsg, base64Encode(hash)...)
	return b64HashedMsg, nil
}

func (box Box) Base64VerifyHash(b64HashedMsg []byte) (msg []byte, err error) {
	i := bytes.Index(b64HashedMsg, []byte{'~'})
	if i < 0 {
		return nil, ErrHashedMsgInvalid
	}
	msg, err = base64Decode(b64HashedMsg[:i])
	if err != nil {
		return nil, err
	}
	hash, err := base64Decode(b64HashedMsg[i+1:])
	if err != nil {
		return nil, err
	}
	err = box.VerifyHash(msg, hash)
	if err != nil {
		return nil, err
	}
	return msg, nil
}
