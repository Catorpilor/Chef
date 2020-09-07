package ethChef

import (
	"bytes"
	"io/ioutil"

	"github.com/BurntSushi/toml"
)

// Config is the structure of the configuartion file
type Config struct {
	Server struct {
		Addr string `toml:"addr"`
	}
	Redis struct {
		Addr    string `toml:"addr"`
		MaxIdle int    `toml:"maxIdle"`
	}
	Telegram struct {
		Token   string `toml:"token"`
		Timeout int    `toml:"timeout"`
	}
	Etherscan struct {
		Key      string `toml:"key"`
		Interval int    `toml:"interval"`
	}
}

// Load loads Configuration from path
func Load(path string, conf *Config) error {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	_, err = toml.DecodeReader(bytes.NewReader(b), conf)
	return err
}
