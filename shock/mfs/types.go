package mfs

import (
	"github.com/ipfs/boxo/mfs"
)

type GetRoot func(ns string) (*mfs.Root, error)
