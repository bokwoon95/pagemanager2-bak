package pagemanager

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/bokwoon95/erro"
	"golang.org/x/crypto/argon2"
)

type keyDerivation struct {
	// params used to derive the key
	argon2Version int
	memory        uint32
	time          uint32
	threads       uint8
	keyLen        uint32
	salt          []byte
	// the derived key
	key []byte
}

func (kd keyDerivation) Marshal() string {
	s := "$argon2id$v=" + strconv.Itoa(kd.argon2Version) +
		"$m=" + strconv.FormatUint(uint64(kd.memory), 10) +
		",t=" + strconv.FormatUint(uint64(kd.time), 10) +
		",p=" + strconv.FormatUint(uint64(kd.threads), 10) +
		",l=" + strconv.FormatUint(uint64(kd.keyLen), 10) +
		"$" + base64.RawURLEncoding.EncodeToString(kd.salt) + "$"
	if len(kd.key) > 0 {
		s += base64.RawURLEncoding.EncodeToString(kd.key)
	}
	return s
}

func (kd keyDerivation) MarshalParams() string {
	return "$argon2id$v=" + strconv.Itoa(kd.argon2Version) +
		"$m=" + strconv.FormatUint(uint64(kd.memory), 10) +
		",t=" + strconv.FormatUint(uint64(kd.time), 10) +
		",p=" + strconv.FormatUint(uint64(kd.threads), 10) +
		",l=" + strconv.FormatUint(uint64(kd.keyLen), 10) +
		"$" + base64.RawURLEncoding.EncodeToString(kd.salt) + "$"
}

func (kd *keyDerivation) Unmarshal(s string) error {
	// parts[0] = empty string
	// parts[1] = argon2id
	// parts[2] = v=%d
	// parts[3] = m=%d,t=%d,p=%d,l=%d
	// parts[4] = base64 URL encoded salt
	// parts[5] = base64 URL encoded key (can be empty, which indicates that key should be re-derived from the above params)
	var err error
	parts := strings.Split(s, "$")
	kd.argon2Version, err = strconv.Atoi(parts[2])
	if err != nil {
		return erro.Wrap(err)
	}
	_, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d,l=%d", &kd.memory, &kd.time, &kd.threads, &kd.keyLen)
	if err != nil {
		return erro.Wrap(err)
	}
	kd.salt, err = base64.RawURLEncoding.DecodeString(parts[4])
	if err != nil {
		return erro.Wrap(err)
	}
	if len(parts[5]) > 0 {
		kd.key, err = base64.RawURLEncoding.DecodeString(parts[5])
		if err != nil {
			return erro.Wrap(err)
		}
	}
	return nil
}

func deriveKeyFromPassword(password string) (keyDerivation, error) {
	kd := keyDerivation{
		argon2Version: argon2.Version,
		memory:        63 * 1024,
		time:          1,
		threads:       4,
		keyLen:        32,
		salt:          make([]byte, 16),
	}
	_, err := rand.Read(kd.salt)
	if err != nil {
		return kd, erro.Wrap(err)
	}
	kd.key = argon2.IDKey([]byte(password), kd.salt, kd.time, kd.memory, kd.threads, kd.keyLen)
	return kd, nil
}

type keyDerivationParams struct {
	argon2Version int
	memory        uint32
	time          uint32
	threads       uint8
	keyLen        uint32
	salt          []byte
}

func (kd keyDerivationParams) deriveKey() {
}

func generateFromPassword() {
}

func compareHashAndPassword() {
}
