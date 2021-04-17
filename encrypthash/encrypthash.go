package encrypthash

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"

	"github.com/bokwoon95/erro"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/nacl/secretbox"
)

type Encrypter interface {
	Encrypt(plaintext []byte) (ciphertext []byte, err error)
	Decrypt(ciphertext []byte) (plaintext []byte, err error)
}

type Hasher interface {
	Hash(data []byte) (hash []byte, err error)
	VerifyHash(data []byte, hash []byte) error
}

type Blackbox struct {
	key        []byte
	getKeys    func() (keys [][]byte, err error)
	processKey func(input []byte) (output []byte, err error)
}

var _ Encrypter = &Blackbox{}
var _ Hasher = &Blackbox{}

func New(key []byte, getKeys func() (keys [][]byte, err error), processKey func(input []byte) (output []byte, err error)) (*Blackbox, error) {
	if len(key) == 0 && getKeys == nil {
		return nil, erro.Wrap(fmt.Errorf("Either keys or getKeys function must be non-nil"))
	}
	return &Blackbox{key: key, getKeys: getKeys}, nil
}

func (box *Blackbox) Encrypt(plaintext []byte) (ciphertext []byte, err error) {
	const nonceSize = 24
	var key []byte
	if box.getKeys != nil {
		var keys [][]byte
		keys, err = box.getKeys()
		if err != nil {
			return nil, erro.Wrap(err)
		}
		if len(keys) == 0 {
			return nil, erro.Wrap(fmt.Errorf("no key found"))
		}
		key = keys[0]
	} else {
		if len(box.key) == 0 {
			return nil, erro.Wrap(fmt.Errorf("no key found"))
		}
		key = box.key
	}
	if box.processKey != nil {
		key, err = box.processKey(key)
		if err != nil {
			return nil, erro.Wrap(err)
		}
	}
	hashedKey := blake2b.Sum512(key)
	var hashKeyUpper [32]byte
	copy(hashKeyUpper[:], hashedKey[:32])
	var nonce [nonceSize]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, erro.Wrap(err)
	}
	ciphertext = secretbox.Seal(nonce[:], plaintext, &nonce, &hashKeyUpper)
	return ciphertext, nil
}

func (box *Blackbox) Decrypt(ciphertext []byte) (plaintext []byte, err error) {
	const nonceSize = 24
	var keys [][]byte
	if box.getKeys != nil {
		keys, err = box.getKeys()
		if err != nil {
			return nil, erro.Wrap(err)
		}
		if len(keys) == 0 {
			return nil, erro.Wrap(fmt.Errorf("no key found"))
		}
	} else {
		if len(box.key) == 0 {
			return nil, erro.Wrap(fmt.Errorf("no key found"))
		}
		keys = [][]byte{box.key}
	}
	for _, key := range keys {
		if box.processKey != nil {
			key, err = box.processKey(key)
			if err != nil {
				return nil, erro.Wrap(err)
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
	return nil, erro.Wrap(fmt.Errorf("decryption error"))
}

func (box *Blackbox) Hash(msg []byte) (hash []byte, err error) {
	var key []byte
	if box.getKeys != nil {
		var keys [][]byte
		keys, err = box.getKeys()
		if err != nil {
			return nil, erro.Wrap(err)
		}
		if len(keys) == 0 {
			return nil, erro.Wrap(fmt.Errorf("no key found"))
		}
		key = keys[0]
	} else {
		if len(box.key) == 0 {
			return nil, erro.Wrap(fmt.Errorf("no key found"))
		}
		key = box.key
	}
	if box.processKey != nil {
		key, err = box.processKey(key)
		if err != nil {
			return nil, erro.Wrap(err)
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

func (box *Blackbox) VerifyHash(msg []byte, hash []byte) error {
	var err error
	var keys [][]byte
	if box.getKeys != nil {
		keys, err = box.getKeys()
		if err != nil {
			return erro.Wrap(err)
		}
		if len(keys) == 0 {
			return erro.Wrap(fmt.Errorf("no key found"))
		}
	} else {
		if len(box.key) == 0 {
			return erro.Wrap(fmt.Errorf("no key found"))
		}
		keys = [][]byte{box.key}
	}
	for _, key := range keys {
		if box.processKey != nil {
			key, err = box.processKey(key)
			if err != nil {
				return erro.Wrap(err)
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
	return erro.Wrap(fmt.Errorf("hash not valid"))
}

func (box *Blackbox) Base64Encrypt(plaintext []byte) (b64Ciphertext []byte, err error) {
	ciphertext, err := box.Encrypt(plaintext)
	if err != nil {
		return nil, erro.Wrap(err)
	}
	b64Ciphertext = make([]byte, base64.RawURLEncoding.EncodedLen(len(ciphertext)))
	base64.RawURLEncoding.Encode(b64Ciphertext, ciphertext)
	return b64Ciphertext, nil
}

func (box *Blackbox) Base64Decrypt(b64Ciphertext []byte) (plaintext []byte, err error) {
	ciphertext := make([]byte, base64.RawURLEncoding.DecodedLen(len(b64Ciphertext)))
	n, err := base64.RawURLEncoding.Decode(ciphertext, b64Ciphertext)
	if err != nil {
		return nil, erro.Wrap(err)
	}
	ciphertext = ciphertext[:n]
	plaintext, err = box.Decrypt(ciphertext)
	if err != nil {
		return nil, erro.Wrap(err)
	}
	return plaintext, nil
}

func (box *Blackbox) Base64Hash(msg []byte) (b64HashedMsg []byte, err error) {
	hash, err := box.Hash(msg)
	if err != nil {
		return nil, erro.Wrap(err)
	}
	b64Msg := make([]byte, base64.RawURLEncoding.EncodedLen(len(msg)))
	base64.RawURLEncoding.Encode(b64Msg, msg)
	b64Hash := make([]byte, base64.RawURLEncoding.EncodedLen(len(hash)))
	base64.RawURLEncoding.Encode(b64Hash, hash)
	b64HashedMsg = append(b64HashedMsg, b64Msg...)
	b64HashedMsg = append(b64HashedMsg, '.')
	b64HashedMsg = append(b64HashedMsg, b64Hash...)
	return b64HashedMsg, nil
}

func (box *Blackbox) Base64VerifyHash(b64HashedMsg []byte) (msg []byte, err error) {
	dotIndex := -1
	for i, c := range b64HashedMsg {
		if c == '.' {
			dotIndex = i
			break
		}
	}
	if dotIndex < 0 {
		return nil, erro.Wrap(fmt.Errorf("invalid b64HashedMsg"))
	}
	b64Msg := b64HashedMsg[:dotIndex]
	msg = make([]byte, base64.RawURLEncoding.DecodedLen(len(b64Msg)))
	n, err := base64.RawURLEncoding.Decode(msg, b64Msg)
	if err != nil {
		return nil, erro.Wrap(err)
	}
	msg = msg[:n]
	b64Hash := b64HashedMsg[dotIndex+1:]
	hash := make([]byte, base64.RawURLEncoding.DecodedLen(len(b64Hash)))
	n, err = base64.RawURLEncoding.Decode(hash, b64Hash)
	if err != nil {
		return nil, erro.Wrap(err)
	}
	err = box.VerifyHash(msg, hash)
	if err != nil {
		return nil, erro.Wrap(err)
	}
	return msg, nil
}
