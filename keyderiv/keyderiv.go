// Package keyderiv is a key derivation wrapper around argon2id.
package keyderiv

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

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

type Params struct {
	Argon2Version int
	Memory        uint32
	Time          uint32
	Threads       uint8
	KeyLen        uint32
	Salt          []byte
}

func NewParams() (Params, error) {
	p := Params{
		Argon2Version: argon2.Version,
		Memory:        63 * 1024,
		Time:          1,
		Threads:       4,
		KeyLen:        32,
		Salt:          make([]byte, 16),
	}
	_, err := rand.Read(p.Salt)
	return p, err
}

func (p Params) String() string {
	b, _ := p.MarshalBinary()
	return string(b)
}

func (p Params) MarshalBinary() (data []byte, err error) {
	var buf []byte
	// version
	buf = append(buf, "$argon2id$v="...)
	buf = strconv.AppendInt(buf, int64(p.Argon2Version), 10)
	// memory
	buf = append(buf, "$m="...)
	buf = strconv.AppendUint(buf, uint64(p.Memory), 10)
	// time
	buf = append(buf, ",t="...)
	buf = strconv.AppendUint(buf, uint64(p.Time), 10)
	// threads
	buf = append(buf, ",p="...)
	buf = strconv.AppendUint(buf, uint64(p.Threads), 10)
	// keyLen
	buf = append(buf, ",l="...)
	buf = strconv.AppendUint(buf, uint64(p.KeyLen), 10)
	// salt
	buf = append(buf, '$')
	buf = append(buf, base64Encode(p.Salt)...)
	buf = append(buf, '$')
	return buf, nil
}

func (p *Params) UnmarshalBinary(data []byte) error {
	// parts[0] = empty string
	// parts[1] = argon2id
	// parts[2] = v=%d
	// parts[3] = m=%d,t=%d,p=%d,l=%d
	// parts[4] = base64 URL encoded salt
	var err error
	parts := strings.Split(string(data), "$")
	_, err = fmt.Sscanf(parts[2], "v=%d", &p.Argon2Version)
	if err != nil {
		return err
	}
	_, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d,l=%d", &p.Memory, &p.Time, &p.Threads, &p.KeyLen)
	if err != nil {
		return err
	}
	p.Salt, err = base64Decode([]byte(parts[4]))
	if err != nil {
		return err
	}
	return nil
}

func (p Params) DeriveKey(password []byte) []byte {
	return argon2.IDKey(password, p.Salt, p.Time, p.Memory, p.Threads, p.KeyLen)
}

func GenerateFromPassword(password []byte) (passwordHash []byte, err error) {
	params, err := NewParams()
	if err != nil {
		return nil, err
	}
	passwordHash, err = params.MarshalBinary()
	if err != nil {
		return nil, err
	}
	passwordHash = append(passwordHash, base64Encode(params.DeriveKey(password))...)
	return passwordHash, nil
}

func CompareHashAndPassword(passwordHash []byte, password []byte) error {
	i := bytes.LastIndex(passwordHash, []byte("$"))
	if i < 0 {
		return fmt.Errorf("invalid hashedPassword")
	}
	var p Params
	err := p.UnmarshalBinary(passwordHash[:i+1])
	if err != nil {
		return err
	}
	derivedKey := p.DeriveKey(password)
	providedKey, err := base64Decode(passwordHash[i+1:])
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(providedKey, derivedKey) != 1 {
		return fmt.Errorf("incorrect password")
	}
	return nil
}
