package sds

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWallet_AddressOK(t *testing.T) {
	wallet, _ := NewSdsWallet("0x5239ac94f2bec8665ffefee845e9b6e398820082d3e584c7a22bfdc2ba17120c")
	fmt.Println(wallet.GetAddress())
	assert.Equal(t, wallet.GetAddress(), "st1wfzldws7aaqxga4299szdp47y95pj3zhg0mcrt")
}
