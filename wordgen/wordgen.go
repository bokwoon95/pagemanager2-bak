// Package wordgen encodes arbitrary byte slices into a list of words
package wordgen

import (
	"crypto/rand"
	_ "embed"
	"fmt"
	"math"
	"math/big"
	"strings"
)

//go:embed wordlist.txt
var wordfile string

var wordlist []string = func() []string {
	var words []string
	var buf []rune
	for _, c := range wordfile {
		switch c {
		case '\r':
			continue
		case '\n':
			word := strings.TrimSpace(string(buf))
			buf = buf[:0]
			if word == "" {
				continue
			}
			words = append(words, word)
		default:
			if len(words) >= 1<<13 {
				break
			}
			buf = append(buf, c)
		}
	}
	if len(words) < 1<<13 {
		panic(fmt.Sprintf("wordlist.txt too small: 8192 (2^13) words expected, found %d", len(words)))
	}
	return words
}()

func Encode(b []byte) []string {
	var words []string
	bitmask := new(big.Int).SetUint64(1<<13 - 1)
	bignum := new(big.Int).SetBytes(b)
	index := new(big.Int)
	for bignum.Uint64() > 0 {
		index.SetUint64(0).And(bignum, bitmask)
		words = append(words, wordlist[index.Uint64()])
		bignum.Rsh(bignum, 13)
	}
	return words
}

func Words(n int) ([]string, error) {
	numbytes := int(math.Ceil((13 * float64(n)) / 8))
	extrabits := (8 * numbytes) - (13 * n)
	b := make([]byte, numbytes)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	b[0] = b[0] & (1<<(8-extrabits) - 1)
	return Encode(b), nil
}
