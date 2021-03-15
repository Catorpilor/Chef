package ethscan

import "testing"

func initWithTesting(t *testing.T) *Client {
	key := "your token"
	client := NewCtl(nil, key)
	t.Cleanup(func() {
		client = nil
	})
	return client
}

func TestClient_QueryTokenTxWithValues(t *testing.T) {
	st := []struct {
		name       string
		addr       string
		startBlock string
		exp        *Resp
		expErr     error
	}{
		{"case1", "0x19020bbb917199db899B64FdC43bD0E380C35a4D", "12038400", &Resp{}, nil},
		{"case2", "0xe9c790e8fde820ded558a4771b72eec916c04763", "", &Resp{}, nil},
	}
	client := initWithTesting(t)
	for _, tt := range st {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.QueryTokenTxWithValues(tt.addr, tt.startBlock)
			if err != tt.expErr {
				t.Fatalf("wanted err: %v but got %v", tt.expErr, err)
			}
			t.Logf("got resp %#v", resp)
		})
	}
}
