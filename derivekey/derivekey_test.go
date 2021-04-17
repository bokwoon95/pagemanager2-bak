package derivekey

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/bokwoon95/pagemanager/testutil"
)

func Test_Password(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		const password = "password"
		is := testutil.New(t, testutil.Parallel)
		hashedPassword, err := GenerateFromPassword([]byte(password))
		is.NoErr(err)
		fmt.Println(string(hashedPassword))
		err = CompareHashAndPassword(hashedPassword, []byte(password))
		is.NoErr(err)
	})
}

func Test_KeyDerivation(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		const password = "password"
		is := testutil.New(t, testutil.Parallel)
		params, err := NewParams()
		is.NoErr(err)
		fmt.Printf("%+v\n", params)
		key := params.DeriveKey([]byte(password))
		var params2 Params
		err = params2.UnmarshalBinary([]byte(params.String()))
		is.NoErr(err)
		key2 := params2.DeriveKey([]byte(password))
		fmt.Printf("key : %s\n", hex.EncodeToString(key))
		fmt.Printf("key2: %s\n", hex.EncodeToString(key2))
		is.Equal(key, key2)
	})
}
