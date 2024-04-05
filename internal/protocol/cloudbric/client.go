package cloudbric

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type serverList struct {
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Data    []*serverInfo `json:"data"`
}

type serverInfo struct {
	IDX         string `json:"idx"`
	ServerName  string `json:"server_name"`
	CountryCode string `json:"country_code"`
	PublicIP    string `json:"public_ip"`
	Port        string `json:"port"`
	DNS         string `json:"dns"`
	PublicKey   string `json:"public_key"`
	Open        string `json:"open"`
	Alive       string `json:"alive"`
	ClientCount string `json:"client_cnt"`
}

type connectStatus struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

var (
	deviceuuid = uuid.New()
	host       = "https://vpn-controller.cloudbric.com"
	client     = http.DefaultClient
	headers    = map[string]string{
		"Content-Type":    "application/json",
		"Lang":            "en",
		"Connection":      "keep-alive",
		"Accept":          "*/*",
		"User-Agent":      "CloudbricVPN/1.1.0 (com.cloudbric.vpnagent; build:202312211016; macOS 14.1.2) Alamofire/5.6.4",
		"Accept-Language": "en-US;q=1.0, ru-US;q=0.9",
		"Accept-Encoding": "br;q=1.0, gzip;q=0.9, deflate;q=0.8",
	}
)

func listServers(ctx context.Context) (*serverList, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, host+"/server/list", http.NoBody)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	rsp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	var servers serverList

	if err := json.NewDecoder(rsp.Body).Decode(&servers); err != nil {
		return nil, err
	}

	return &servers, nil
}

func activateConn(ctx context.Context, s *serverInfo, id, key string) (*connectStatus, error) {
	vals := url.Values{}

	pkey, err := wgtypes.ParseKey(key)
	if err != nil {
		return nil, err
	}

	vals.Add("app_version", "1.1.0")
	vals.Add("device_id", strings.ToUpper(id))
	vals.Add("os_type", "MAC")
	vals.Add("public_key", pkey.PublicKey().String())
	vals.Add("vpn_server_idx", s.IDX)

	body := bytes.NewBufferString(vals.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, host+"/connect/active", body)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	rsp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	var connStatus connectStatus

	if err := json.NewDecoder(rsp.Body).Decode(&connStatus); err != nil {
		return nil, err
	}

	return &connStatus, nil
}

func deactiveConn() (*connectStatus, error) {
	vals := url.Values{}

	vals.Add("device_id", deviceuuid.String())

	body := bytes.NewBufferString(vals.Encode())

	//nolint:noctx // call after deadline
	req, err := http.NewRequest(http.MethodPost, host+"/connect/deactive", body)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	rsp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	var connStatus connectStatus

	if err := json.NewDecoder(rsp.Body).Decode(&connStatus); err != nil {
		return nil, err
	}

	return &connStatus, nil
}
