package sds

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	logging "github.com/ipfs/go-log/v2"
	fwtypes "github.com/stratosnet/sds/framework/types"
	rpc_api "github.com/stratosnet/sds/pp/api/rpc"
	"github.com/stratosnet/sds/sds-msg/protos"
)

var logRpc = logging.Logger("sds/rpc")

type jsonrpcMessage struct {
	Version string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e jsonrpcError) Error() string {
	return e.String()
}

func (e *jsonrpcError) String() string {
	return fmt.Sprintf("jsonrpc request error: code: %d; message: %s", e.Code, e.Message)
}

type jsonrpcResponse struct {
	Version string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
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
		logRpc.Error("json marshal error", e)
		return nil
	}
	return r
}

var timeout = 30 * time.Second

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
		logRpc.Error("failed marshal param for " + method)
		return err
	}

	// wrap to the json-rpc message
	request := wrapJsonRpc(method, pm)

	if len(request) < 300 {
		logRpc.Debug("--> ", string(request))
	} else {
		logRpc.Debug("--> ", string(request[:230]), "... \"}]}")
	}

	// http post
	req, err := http.NewRequest("POST", rpc.httpRpcUrl, bytes.NewBuffer(request))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: timeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	body, _ := io.ReadAll(resp.Body)
	if len(body) < 300 {
		logRpc.Debug("<-- ", string(body))
	} else {
		logRpc.Debug("<-- ", string(body[:230]), "... \"}]}")
	}

	resp.Body.Close()

	if len(body) == 0 {
		logRpc.Error("empty body after read buffer")
		return fmt.Errorf("empty response body")
	}

	// handle rsp
	var rsp jsonrpcResponse
	err = json.Unmarshal(body, &rsp)
	if err != nil {
		return err
	}

	logRpc.Debugf("got json response: %+v", rsp)

	if rsp.Error != nil {
		return rsp.Error
	}

	err = json.Unmarshal(rsp.Result, &res)
	if err != nil {
		logRpc.Error("unmarshal failed")
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

func (rpc *Rpc) UploadSign(wallet *SdsWallet, sn, fileHash string) (*rpc_api.Result, error) {
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

	req := rpc_api.ParamUploadSign{
		FileHash: fileHash,
		Signature: rpc_api.Signature{
			Address:   wallet.GetAddress(),
			Pubkey:    wpk,
			Signature: hex.EncodeToString(sign),
		},
		ReqTime:        nowSec,
		SequenceNumber: sn,
	}

	var res rpc_api.Result
	err = rpc.sendRequest("user_uploadSign", req, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (rpc *Rpc) GetFileStatus(wallet *SdsWallet, fileHash string) (*rpc_api.FileStatusResult, error) {
	nowSec := time.Now().Unix()
	// signature
	sign, err := wallet.SignGetFileStatus(fileHash)
	if err != nil {
		return nil, err
	}
	wpk, err := wallet.GetBech32PubKey()
	if err != nil {
		return nil, err
	}

	req := rpc_api.ParamGetFileStatus{
		FileHash: fileHash,
		Signature: rpc_api.Signature{
			Address:   wallet.GetAddress(),
			Pubkey:    wpk,
			Signature: hex.EncodeToString(sign),
		},
		ReqTime: nowSec,
	}

	res := rpc_api.FileStatusResult{FileUploadState: protos.FileUploadState_UNKNOWN}
	err = rpc.sendRequest("user_getFileStatus", req, &res)
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

func (rpc *Rpc) RequestShare(wallet *SdsWallet, fileHash string, cid *string) (*rpc_api.FileShareResult, error) {
	nowSec := time.Now().Unix()
	// signature
	sign, err := wallet.SignCreateShareLink(fileHash)
	if err != nil {
		return nil, err
	}
	wpk, err := wallet.GetBech32PubKey()
	if err != nil {
		return nil, err
	}

	req := rpc_api.ParamReqShareFile{
		FileHash: fileHash,
		Signature: rpc_api.Signature{
			Address:   wallet.GetAddress(),
			Pubkey:    wpk,
			Signature: hex.EncodeToString(sign),
		},
		ReqTime: nowSec,
	}

	if cid != nil {
		req.IpfsCid = *cid
	}

	var res rpc_api.FileShareResult
	err = rpc.sendRequest("user_requestShare", req, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (rpc *Rpc) GetShared(wallet *SdsWallet, sn string, parsedLink *fwtypes.ShareDataMeshId) (*rpc_api.Result, error) {
	nowSec := time.Now().Unix()

	// signature
	sign, err := wallet.SignGetShareLink(sn, parsedLink.Link)
	if err != nil {
		return nil, err
	}
	wpk, err := wallet.GetBech32PubKey()
	if err != nil {
		return nil, err
	}

	req := rpc_api.ParamReqGetShared{
		Signature: rpc_api.Signature{
			Address:   wallet.GetAddress(),
			Pubkey:    wpk,
			Signature: hex.EncodeToString(sign),
		},
		ReqTime:   nowSec,
		ShareLink: parsedLink.String(),
	}

	var res rpc_api.Result
	err = rpc.sendRequest("user_requestGetShared", req, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}
