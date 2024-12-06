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
func (api *SdsAPI) Link(ctx context.Context, cid cid.Cid, fileHash string, opts ...options.SdsOption) (files.File, error) {
	settings, err := options.SdsOptions(opts...)
	if err != nil {
		return nil, err
	}

	mapFile, err := sds.NewSdsFile(cid, fileHash)
	if err != nil {
		return nil, err
	}

	go api.sdsFetcher.CreateShareLink(settings.PrivKey, fileHash, cid.String())

	f, ok := mapFile.(files.File)
	if !ok {
		return nil, fmt.Errorf("not a file")
	}
	return f, nil
}

// Add imports the data from the reader into sds store chunks
func (api *SdsAPI) Upload(ctx context.Context, file_ files.File, opts ...options.SdsOption) (string, error) {
	settings, err := options.SdsOptions(opts...)
	if err != nil {
		return "", err
	}

	fileData, err := io.ReadAll(file_)
	if err != nil {
		return "", err
	}

	return api.sdsFetcher.Upload(settings.PrivKey, fileData)
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
	if err != nil {
		return path.ImmutablePath{}, err
	}

	ip, err := path.NewPath("/ipfs/" + originalCid.String())
	if err != nil {
		return path.ImmutablePath{}, err
	}
	return path.NewImmutablePath(ip)
}

func (api *SdsAPI) Download(ctx context.Context, p path.Path, opts ...options.SdsOption) (files.File, error) {
	settings, err := options.SdsOptions(opts...)
	if err != nil {
		return nil, err
	}

	shareLink := fwtypes.SetShareLink(p.Segments()[1], "")
	fileData, err := api.sdsFetcher.DownloadFromShare(settings.PrivKey, shareLink.String())
	if err != nil {
		return nil, err
	}

	rfc := files.NewBytesFile(fileData)

	return rfc, nil
}
