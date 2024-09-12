package sds

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ipfs/go-cid"
	mbase "github.com/multiformats/go-multibase"
	mh "github.com/multiformats/go-multihash"
	"github.com/stratosnet/sds/framework/crypto"

	rpc_api "github.com/stratosnet/sds/pp/api/rpc"
)

type jsonrpcMessage struct {
	Version string          `json:"jsonrpc,omitempty"`
	ID      int             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

func wrapJsonRpc(method string, param []byte) []byte {
	// compose json-rpc request
	request := &jsonrpcMessage{
		Version: "2.0",
		ID:      1,
		Method:  method,
		Params:  json.RawMessage(param),
	}
	r, e := json.Marshal(request)
	if e != nil {
		logger.Error("json marshal error", e)
		return nil
	}
	return r
}

func CreateFileHash(fileData []byte) string {
	sliceKeccak256, _ := mh.SumStream(bytes.NewReader(fileData), mh.KECCAK_256, 20)
	data := append([]byte(""), sliceKeccak256...)
	kHash, _ := mh.Sum(data, mh.KECCAK_256, 20)
	fileCid := cid.NewCidV1(uint64(crypto.SDS_CODEC), kHash)
	encoder, _ := mbase.NewEncoder(mbase.Base32hex)
	return fileCid.Encode(encoder)
}

type Rpc struct {
	httpRpcUrl string
}

func NewRpc(httpRpcUrl string) (*Rpc, error) {
	return &Rpc{
		httpRpcUrl: httpRpcUrl,
	}, nil
}

func (rpc *Rpc) sendRequest(method string, param any, res any) error {
	var params []any
	params = append(params, param)
	pm, err := json.Marshal(params)
	if err != nil {
		logger.Error("failed marshal param for " + method)
		return err
	}

	// wrap to the json-rpc message
	request := wrapJsonRpc(method, pm)

	if len(request) < 300 {
		logger.Debug("--> ", string(request))
	} else {
		logger.Debug("--> ", string(request[:230]), "... \"}]}")
	}

	// http post
	req, err := http.NewRequest("POST", rpc.httpRpcUrl, bytes.NewBuffer(request))
	if err != nil {
		return err
	}
	// req.Header.Set("X-Custom-Header", "myvalue")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	body, _ := io.ReadAll(resp.Body)
	if len(body) < 300 {
		logger.Debug("<-- ", string(body))
	} else {
		logger.Debug("<-- ", string(body[:230]), "... \"}]}")
	}

	resp.Body.Close()

	if len(body) == 0 {
		logger.Error("emptry body after read buffer")
		return fmt.Errorf("empty response body")
	}

	// handle rsp
	var rsp jsonrpcMessage
	err = json.Unmarshal(body, &rsp)
	if err != nil {
		return err
	}

	err = json.Unmarshal(rsp.Result, &res)
	if err != nil {
		logger.Error("unmarshal failed")
		return err
	}
	return nil
}

func (rpc *Rpc) GetOzone(wallet *SdsWallet) (*rpc_api.GetOzoneResult, error) {
	req := &rpc_api.ParamReqGetOzone{
		WalletAddr: wallet.GetAddress(),
	}

	var res rpc_api.GetOzoneResult

	err := rpc.sendRequest("user_requestGetOzone", req, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (rpc *Rpc) RequestUpload(wallet *SdsWallet, sn, fileName, fileHash string, fileSize int) (*rpc_api.Result, error) {
	nowSec := time.Now().Unix()

	sign, err := wallet.SignFileUpload(sn, fileHash)
	if err != nil {
		return nil, err
	}
	wpk, err := wallet.GetBech32PubKey()
	if err != nil {
		return nil, err
	}

	req := &rpc_api.ParamReqUploadFile{
		FileName: fileName,
		FileHash: fileHash,
		FileSize: fileSize,
		Signature: rpc_api.Signature{
			Address:   wallet.GetAddress(),
			Pubkey:    wpk,
			Signature: hex.EncodeToString(sign),
		},
		DesiredTier:     1,
		AllowHigherTier: true,
		ReqTime:         nowSec,
		SequenceNumber:  sn,
	}

	var res rpc_api.Result
	err = rpc.sendRequest("user_requestUpload", req, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (rpc *Rpc) UploadData(wallet *SdsWallet, sn, fileHash string, fileChunk string) (*rpc_api.Result, error) {
	nowSec := time.Now().Unix()
	// signature
	sign, err := wallet.SignFileUpload(sn, fileHash)
	if err != nil {
		return nil, err
	}
	wpk, err := wallet.GetBech32PubKey()
	if err != nil {
		return nil, err
	}

	req := rpc_api.ParamUploadData{
		FileHash: fileHash,
		Data:     fileChunk,
		Signature: rpc_api.Signature{
			Address:   wallet.GetAddress(),
			Pubkey:    wpk,
			Signature: hex.EncodeToString(sign),
		},
		ReqTime:        nowSec,
		SequenceNumber: sn,
	}

	var res rpc_api.Result
	err = rpc.sendRequest("user_uploadData", req, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (rpc *Rpc) RequestDownload(wallet *SdsWallet, sn, fileHash string) (*rpc_api.Result, error) {
	nowSec := time.Now().Unix()
	// signature
	sign, err := wallet.SignDownloadData(sn, fileHash)
	if err != nil {
		return nil, err
	}
	wpk, err := wallet.GetBech32PubKey()
	if err != nil {
		return nil, err
	}

	req := rpc_api.ParamReqDownloadFile{
		FileHandle: "sdm://" + wallet.GetAddress() + "/" + fileHash,
		Signature: rpc_api.Signature{
			Address:   wallet.GetAddress(),
			Pubkey:    wpk,
			Signature: hex.EncodeToString(sign),
		},
		ReqTime: nowSec,
	}

	var res rpc_api.Result
	err = rpc.sendRequest("user_requestDownload", req, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (rpc *Rpc) DownloadData(wallet *SdsWallet, reqid, fileHash string) (*rpc_api.Result, error) {
	req := rpc_api.ParamDownloadData{
		ReqId:    reqid,
		FileHash: fileHash,
	}

	var res rpc_api.Result
	err := rpc.sendRequest("user_downloadData", req, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (rpc *Rpc) DownloadedFileInfo(wallet *SdsWallet, reqid, fileHash string, fileSize uint64) (*rpc_api.Result, error) {
	// param
	req := rpc_api.ParamDownloadFileInfo{
		FileHash: fileHash,
		FileSize: fileSize,
		ReqId:    reqid,
	}

	var res rpc_api.Result
	err := rpc.sendRequest("user_downloadedFileInfo", req, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}
