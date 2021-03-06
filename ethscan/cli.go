package ethscan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

type Client struct {
	client *http.Client
	apiKey string
}

var defaultClient = &http.Client{Timeout: 20 * time.Second}

// NewCtl create a etherscan client
func NewCtl(client *http.Client, key string) *Client {
	if client == nil {
		client = defaultClient
	}
	return &Client{client: client, apiKey: key}
}

type Tx struct {
	BlockNumber  string `json:"blockNumber,omitempty"`
	TimeStamp    string `json:"timeStamp,omitempty"`
	Hash         string `json:"hash,omitempty"`
	BlockHash    string `json:"blockHash,omitempty"`
	From         string `json:"from,omitempty"`
	To           string `json:"to,omitempty"`
	Value        string `json:"value,omitempty"`
	TokenName    string `json:"tokenName,omitempty"`
	TokenSymbol  string `json:"tokenSymbol,omitempty"`
	TokenDecimal string `json:"tokenDecimal,omitempty"`
	Action       string `json:"-"`
}

type Resp struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Result  []Tx   `json:"result"`
}

const (
	tokenTxURI    = "https://api.etherscan.io/api?module=account&action=tokentx&address=%s&sort=desc&apikey=%s"
	defaultOffset = "&page=1&offset=20"
)

func (c *Client) callWithURL(url string) (*Resp, error) {
	req, _ := http.NewRequest("GET", url, nil)
	resp, err := c.client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	var res *Resp
	if err != nil {
		log.Infof("calling etherscan api with url:%s got err:%s", url, err.Error())
		return nil, err
	}
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, resp.Body)
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		log.Infof("unmarshal %s got err:%s", buf.String(), err.Error())
		return nil, err
	}
	return res, nil
}

// QueryTokenTxWithValues get erc20 token tx with values
func (c *Client) QueryTokenTxWithValues(addr, startBlock string) (*Resp, error) {
	log.Infof("calling token tx with addr: %s and startBlock: %s\n", addr, startBlock)
	time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
	rawUrl := fmt.Sprintf(tokenTxURI, addr, c.apiKey)
	if startBlock == "" {
		// we use the patination api just retrive latest 20 txs
		rawUrl += defaultOffset
	} else {
		rawUrl += fmt.Sprintf("&startblock=%s&endblock=999999999", startBlock)
	}
	return c.callWithURL(rawUrl)
}
