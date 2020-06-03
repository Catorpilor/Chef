package idena

import (
	"net/http"
	"time"
)

type Client struct {
	client *http.Client
}

var defaultClient = &http.Client{Timeout: 2 * time.Second}

func NewCtl(client *http.Client) *Client {
	return nil
}
