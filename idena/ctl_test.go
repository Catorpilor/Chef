package idena

import "testing"

func initWithTesting(t *testing.T) *Client {
	lc := NewCtl(nil)
	t.Cleanup(func() {
		lc = nil
	})
	return lc
}

func TestValidateAddress(t *testing.T) {
	lc := initWithTesting(t)
	st := []struct {
		name string
		addr string
		exp  bool
	}{
		{"testcase1", "someinvalidaddr", false},
		{"testcase2", "0x8b93bbaf621202e5029a8b7878f549c59c22990f", true},
		{"testcase3", "0x8b93bbaf621202e5029a8b7878f549c59c22990x", false},
	}
	for _, tt := range st {
		t.Run(tt.name, func(t *testing.T) {
			out := lc.ValidateAddress(tt.addr)
			if out != tt.exp {
				t.Fatalf("with input addr:%s wanted %t but got %t", tt.addr, tt.exp, out)
			}
			t.Log("pass")
		})
	}
}
