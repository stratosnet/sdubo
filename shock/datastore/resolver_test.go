package datastore

import (
	"testing"

	ds "github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/assert"
)

func TestJoinKeys(t *testing.T) {
	dsk := ds.NewKey("/local/filesroot")

	key1 := ds.NewKey("test")
	keyR1 := JoinKeys(dsk, key1)

	assert.Equal(t, dsk.String()+key1.String(), keyR1.String())

	key2 := ds.NewKey("")
	keyR2 := JoinKeys(dsk, key2)

	assert.Equal(t, dsk.String(), keyR2.String())

	key3 := ds.NewKey("test/qwe")
	keyR3 := JoinKeys(dsk, key3)

	assert.Equal(t, dsk.String()+key3.String(), keyR3.String())
}
