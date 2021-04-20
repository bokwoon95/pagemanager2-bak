package wordgen

import (
	"fmt"
	"testing"

	"github.com/bokwoon95/pagemanager/testutil"
)

func Test(t *testing.T) {
	is := testutil.New(t)
	words, err := Words(3)
	is.NoErr(err)
	is.Equal(3, len(words))
	fmt.Println(words)
}
