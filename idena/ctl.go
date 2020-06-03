package idena

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

type Client struct {
	client *http.Client
}

var defaultClient = &http.Client{Timeout: 2 * time.Second}

func NewCtl(client *http.Client) *Client {
	if client == nil {
		client = defaultClient
	}
	return &Client{client: client}
}

type Resp struct {
	SuccessResp SuccessResp `json:"result,omitempty"`
	ErrResp     ErrorResp   `json:"error,omitempty"`
	IsValid     bool        `json:"-"`
}

type SuccessResp struct {
	Address            string `json:"address,omitempty"`
	Balance            string `json:"balance,omitempty"`
	Stake              string `json:"stake,omitempty"`
	LastActivity       string `json:"lastActivity,omitempty"`
	Penalty            string `json:"penalty,omitempty"`
	State              string `json:"state,omitempty"`
	TxCount            int    `json:"txCount,omitempty"`
	FlipsCount         int    `json:"flipsCount,omitempty"`
	ReportedFlipsCount int    `json:"reportedFlipsCount,omitempty"`
	Online             bool   `json:"online,omitempty"`
}

type ErrorResp struct {
	Message string `json:"message,omitempty"`
}

const (
	addrURL        = "https://api.idena.org/api/address/"
	onlineIdentity = "https://api.idena.org/api/onlineidentity/"
	addrLen        = 42
)

func (c *Client) callWithURL(url string) (*Resp, error) {
	req, _ := http.NewRequest("GET", url, nil)
	resp, err := c.client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	var res *Resp
	if err != nil {
		log.Infof("calling idena api with url:%s got err:%s", url, err.Error())
		return nil, err
	}
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, resp.Body)
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		log.Infof("unmarshal %s got err:%s", buf.String(), err.Error())
		return nil, err
	}
	if res.ErrResp.Message == "" {
		res.IsValid = true
	}
	return res, nil
}

type OnlineStatus struct {
	LastActivity string `json:"lastActivity,omitempty"`
	Penalty      string `json:"penalty,omitempty"`
	Online       bool   `json:"online,omitempty"`
}

func (os *OnlineStatus) ConstructFromResp(resp SuccessResp) {
	os.LastActivity = resp.LastActivity
	os.Penalty = resp.Penalty
	os.Online = resp.Online
}

func (c *Client) ValidateAddress(addr string) bool {
	if len(addr) != addrLen {
		return false
	}
	res, err := c.callWithURL(addrURL + addr)
	if err != nil {
		return false
	}
	return res.IsValid
}

func (c *Client) CheckOnlineIdentity(addr string) *OnlineStatus {
	res, err := c.callWithURL(onlineIdentity + addr)
	if err != nil {
		return nil
	}
	os := &OnlineStatus{}
	os.ConstructFromResp(res.SuccessResp)
	return os
}
