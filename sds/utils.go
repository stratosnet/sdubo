package sds

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"reflect"
	"unsafe"

	"github.com/ipfs/go-cid"
	mbase "github.com/multiformats/go-multibase"
	mh "github.com/multiformats/go-multihash"
	"github.com/stratosnet/sds/framework/crypto"
)

func randomFileName(size int, ext string) (string, error) {
	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x.%s", b, ext), nil
}

func CreateFileHash(fileData []byte) string {
	sliceKeccak256, _ := mh.SumStream(bytes.NewReader(fileData), mh.KECCAK_256, 20)
	data := append([]byte(""), sliceKeccak256...)
	kHash, _ := mh.Sum(data, mh.KECCAK_256, 20)
	fileCid := cid.NewCidV1(uint64(crypto.SDS_CODEC), kHash)
	encoder, _ := mbase.NewEncoder(mbase.Base32hex)
	return fileCid.Encode(encoder)
}

func getDynamicField(i any, key string) any {
	field := reflect.ValueOf(i).Elem().FieldByName(key)
	// unlock for modification
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface()
}

func setDynamicField(i any, key string, value any) error {
	// Get the reflection value of the struct
	v := reflect.ValueOf(i).Elem()

	// Find the field by name
	field := v.FieldByName(key)
	if !field.IsValid() {
		return fmt.Errorf("no such field: %s", key)
	}

	// Check if the field can be set
	if !field.CanSet() {
		// If the field is unexported, we can use unsafe to modify it
		field = reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
		if !field.CanSet() {
			return fmt.Errorf("cannot set field: %s", key)
		}
	}

	// Set the value to the field
	field.Set(reflect.ValueOf(value))
	return nil
}
