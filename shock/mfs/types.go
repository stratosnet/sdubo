package mfs

import (
	"github.com/ipfs/boxo/mfs"
	datastore "github.com/ipfs/go-datastore"
	format "github.com/ipfs/go-ipld-format"
)

type GetRoot func(ns string) (*mfs.Root, error)

type Repo struct {
	MFSDS datastore.Datastore
	SDSDS datastore.Datastore
	DAG   format.DAGService
}
