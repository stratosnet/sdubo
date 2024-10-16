package coreapi

import (
	"context"
	"fmt"
	"io"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/path"
	cid "github.com/ipfs/go-cid"
	options "github.com/ipfs/kubo/core/coreiface/options"
	"github.com/ipfs/kubo/sds"
	fwtypes "github.com/stratosnet/sds/framework/types"
)

type SdsAPI CoreAPI

// Link a path with sds as share link
func (api *SdsAPI) Link(ctx context.Context, cid cid.Cid, fileHash string, opts ...options.UnixfsAddOption) (files.File, error) {
	mapFile, err := sds.NewSdsFile(cid, fileHash)
	if err != nil {
		return nil, err
	}
	if _, err = api.sdsFetcher.CreateShareLink(fileHash, cid.String()); err != nil {
		return nil, err
	}
	f, ok := mapFile.(files.File)
	if !ok {
		return nil, fmt.Errorf("not a file")
	}
	return f, nil
}

// Add imports the data from the reader into sds store chunks
func (api *SdsAPI) Upload(ctx context.Context, file_ files.File, opts ...options.UnixfsAddOption) (string, error) {
	fileData, err := io.ReadAll(file_)
	if err != nil {
		return "", err
	}

	return api.sdsFetcher.Upload(fileData)
}

func (api *SdsAPI) Parse(ctx context.Context, file_ files.File) (path.ImmutablePath, error) {
	fsize, err := file_.Size()
	if err != nil {
		return path.ImmutablePath{}, err
	}

	fileData := make([]byte, fsize)
	_, err = io.ReadFull(file_, fileData)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	originalCid, err := sds.ParseLink(fileData)
	fmt.Println("Parse originalCid", originalCid)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	ip, err := path.NewPath("/ipfs/" + originalCid.String())
	if err != nil {
		return path.ImmutablePath{}, err
	}
	return path.NewImmutablePath(ip)
}

func (api *SdsAPI) Download(ctx context.Context, p path.Path) (files.File, error) {
	fmt.Println("p.Segments()", p.Segments())
	fmt.Println("p.Segments()[1]", p.Segments()[1])
	shareLink := fwtypes.SetShareLink(p.Segments()[1], "")
	fmt.Println("shareLink", shareLink)
	fileData, err := api.sdsFetcher.DownloadFromShare(shareLink.String())
	if err != nil {
		return nil, err
	}

	rfc := files.NewBytesFile(fileData)

	return rfc, nil
}
