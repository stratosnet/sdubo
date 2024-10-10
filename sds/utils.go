package sds

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"unsafe"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	gocarv2 "github.com/ipld/go-car/v2"
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

// readFile reads the contents of the file from the provided file path.
// If the file exists and has content, it returns the data; otherwise, it returns an error.
func readFile(filePath string) ([]byte, error) {
	// Open the file for reading
	ff, err := os.OpenFile(filePath, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}
	defer ff.Close()

	// Get file info
	fInfo, err := ff.Stat()
	if err != nil {
		return nil, err
	}

	// If the file is empty, return an empty slice or nil
	if fInfo.Size() == 0 {
		return nil, nil
	}

	// Read the file contents
	fileData, err := io.ReadAll(ff)
	if err != nil {
		return nil, err
	}

	return fileData, nil
}

// writeOnly writes the provided byte data to the specified file path.
// It creates or overwrites the file with the data.
func writeOnly(filePath string, data []byte) error {
	// Open the file with write-only and create flags (truncate if exists)
	file, err := os.OpenFile(filePath, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write the data to the file
	_, err = file.Write(data)
	if err != nil {
		return err
	}

	return nil
}

func IsCAR(f files.Node) (bool, error) {
	file, ok := f.(files.File)
	if !ok {
		return false, fmt.Errorf("not a file")
	}

	// TODO: Optimize and use header reading to detect cbor so we do not need to read a whole file
	_, err := gocarv2.NewBlockReader(file)
	if err != nil {
		return false, err
	}

	fs := file.(io.ReadSeeker)
	// we need to seek at initial position as reader not copied during cbor read
	if _, err := fs.Seek(0, io.SeekStart); err != nil {
		return false, err
	}
	return true, nil
}

// ModifySdsCARPath modifies path of root cid from dag in order to get it later from ipfs
// Example:
//
// /ipfs/QmbLQrW85vfWyySX76dwyxxAzz4tcsPk6tgTuLDQjNYxE7/1.txt -> /ipfs/Qmb5WoZiXqWpfHojUf7Yhayracay5TjvCTE4cNAjXwuvVY/1.txt
// means to refer on real cid to match edge file
// othervise do nothing in case of
// /ipfs/QmbLQrW85vfWyySX76dwyxxAzz4tcsPk6tgTuLDQjNYxE7 -> /ipfs/QmbLQrW85vfWyySX76dwyxxAzz4tcsPk6tgTuLDQjNYxE7
// as it is a directory
func ModifySdsCARPath(dstp path.Path, srcp path.Path) (path.Path, error) {
	// for folder + file match
	if len(srcp.Segments()) > 2 {
		c := make([]string, len(srcp.Segments())-2)
		copy(c, srcp.Segments()[2:])

		dstp, err := path.NewPath(filepath.Join(dstp.String(), strings.Join(c, "/")))
		if err != nil {
			return nil, err
		}
		return dstp, nil
	}
	return dstp, nil
}
