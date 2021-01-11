package guard

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/catorpilor/ethChef/ethscan"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/gomodule/redigo/redis"
	log "github.com/sirupsen/logrus"
)

const (
	defaultInterval = 15 * time.Minute
	prefixLen       = 11 // idena:addr: length
)

var (
	unknownAddrErr = errors.New("unknown address, please call /add first.")
)

type Guardian struct {
	bot          *tgbotapi.BotAPI
	watched      map[string]int64
	lastActivity map[string]int64
	alias        map[string]string
	ticker       *time.Ticker
	ethCtl       *ethscan.Client
	redisCtl     *redis.Pool
	lock         sync.RWMutex
}

func New(bot *tgbotapi.BotAPI, ctl *ethscan.Client, redisCtl *redis.Pool, interval int) *Guardian {
	guard := &Guardian{
		bot:          bot,
		watched:      make(map[string]int64),
		lastActivity: make(map[string]int64),
		alias:        make(map[string]string),
		ticker:       time.NewTicker(time.Duration(interval) * time.Second),
		ethCtl:       ctl,
		redisCtl:     redisCtl,
	}
	guard.update()
	return guard
}

func (guard *Guardian) update() {
	c := guard.redisCtl.Get()
	defer c.Close()
	count, err := redis.Int(c.Do("GET", "idena:count"))
	if err != nil {
		if errors.Is(err, redis.ErrNil) {
			return
		}
		log.Infof("get key:idena:count from redis got err:%s", err.Error())
		return
	}
	if count < 1 {
		return
	}
	keys, err := redis.Strings(c.Do("KEYS", "ether:addr:*"))
	if err != nil {
		log.Infof("get keys idena:addr:* failed with err:%s", err.Error())
		return
	}
	activityKeys, err := redis.Strings(c.Do("KEYS", "lastActivity:%s"))
	if err != nil {
		log.Infof("get activityKeys got err:%s", err)
	}
	_ = c.Send("MULTI")
	for _, key := range keys {
		_ = c.Send("GET", key)
	}
	vals, err := redis.Int64s(c.Do("EXEC"))
	if err != nil {
		log.Infof("exec get vals from keys:%v failed %s", keys, err.Error())
		return
	}
	_ = c.Send("MULTI")
	for _, key := range activityKeys {
		_ = c.Send("GET", key)
	}
	activities, err := redis.Strings(c.Do("EXEC"))
	if err != nil {
		log.Infof("multi/exec activityKeys:%v got err:%v", activityKeys, err)
	}
	// get alias back
	_ = c.Send("MULTI")
	for _, key := range keys {
		_ = c.Send("GET", fmt.Sprintf("addr:%s:alias", key[prefixLen:]))
	}
	alias, err := redis.Strings(c.Do("EXEC"))
	if err != nil {
		log.Infof("multi/exec alias got err: %v", err)
	}
	guard.lock.Lock()
	for i := range keys {
		guard.watched[keys[i][prefixLen:]] = vals[i]
		guard.alias[keys[i][prefixLen:]] = alias[i]
	}
	for i := range activityKeys {
		bn, err := strconv.ParseInt(activities[i], 10, 64)
		if err != nil {
			log.Infof("parseInt with arg: %s got err:%v", activities[i], err)
			bn = 0
		}
		guard.lastActivity[keys[i][13:]] = bn
	}
	guard.lock.Unlock()
	log.Infof("udpate watched list:%v, and activityList:%v", guard.watched, guard.lastActivity)
}

func (guard *Guardian) Set(addr, alia string) error {
	if !guard.isWatched(addr) {
		return unknownAddrErr
	}
	guard.lock.Lock()
	defer guard.lock.Unlock()
	guard.alias[addr] = alia
	c := guard.redisCtl.Get()
	defer c.Close()
	_, err := c.Do("SET", fmt.Sprintf("addr:%s:alias", addr), alia)
	return err
}

func (guard *Guardian) isWatched(addr string) bool {
	guard.lock.RLock()
	defer guard.lock.RUnlock()
	if _, exists := guard.watched[addr]; exists {
		return true
	}
	return false
}

func (guard *Guardian) Add(addr string, chatID int64, alias string) {
	if guard.isWatched(addr) {
		return
	}
	c := guard.redisCtl.Get()
	defer c.Close()
	_ = c.Send("MULTI")
	_ = c.Send("INCR", "idena:count")
	_ = c.Send("SET", fmt.Sprintf("ether:addr:%s", addr), strconv.FormatInt(chatID, 10))
	_ = c.Send("SET", fmt.Sprintf("addr:%s:alias", addr), alias)
	if _, err := c.Do("EXEC"); err != nil {
		log.Infof("persist addr:%s and chatID:%d got err:%s", addr, chatID, err.Error())
		// TODO(@catorpilor): add retry logic here.
	}
	guard.lock.Lock()
	guard.watched[addr] = chatID
	guard.alias[addr] = alias
	guard.lock.Unlock()
}

func (guard *Guardian) Delete(addr string) {
	if !guard.isWatched(addr) {
		return
	}
	guard.lock.Lock()
	delete(guard.watched, addr)
	delete(guard.lastActivity, addr)
	guard.lock.Unlock()
	c := guard.redisCtl.Get()
	defer c.Close()
	_ = c.Send("MULTI")
	_ = c.Send("DECR", "idena:count")
	_ = c.Send("DEL", fmt.Sprintf("ether:addr:%s", addr))
	_ = c.Send("DEL", fmt.Sprintf("lastActivity:%s", addr))
	if _, err := c.Do("EXEC"); err != nil {
		log.Infof("delete addr:%s in redis got err:%v", addr, err)
	}
}

func (guard *Guardian) Start() {
	go func() {
		for {
			select {
			case <-guard.ticker.C:
				for addr, chatID := range guard.watched {
					go func(addr string, chatID int64) {
						c := guard.redisCtl.Get()
						defer c.Close()
						var startBlock string
						if guard.lastActivity[addr] != 0 {
							startBlock = strconv.FormatInt(guard.lastActivity[addr], 10)
						}
						rp, err := guard.ethCtl.QueryTokenTxWithValues(addr, startBlock)
						if err != nil {
							log.Infof("querying addr: %s with lastSeen:%s got err:%s", addr, startBlock, err)
							return
						}
						log.Infof("got reply message:%s and status:%s, number of txs: %d", rp.Message, rp.Status, len(rp.Result))
						if rp.Message == "OK" {
							if len(rp.Result) > 0 {
								bn, err := strconv.ParseInt(rp.Result[0].BlockNumber, 10, 64)
								if err != nil {
									log.Infof("parse %s to int got err: %v", rp.Result[0].BlockNumber, err)
									bn = -1
								}
								guard.lastActivity[addr] = bn + 1
								if _, err := c.Do("SET", fmt.Sprintf("lastActivity:%s", addr), rp.Result[0].BlockNumber); err != nil {
									log.Infof("set lastActivity:%s got err:%v", addr, err)
								}
								msg := tgbotapi.NewMessage(chatID, constructWithTx(rp.Result, addr, guard.alias[addr]))
								msg.ParseMode = tgbotapi.ModeMarkdown
								_, err = guard.bot.Send(msg)
								if err != nil {
									log.Infof("send message: %v got  err:%v", msg, err)
									return
								}
							}

						}
					}(addr, chatID)
				}
			}
		}

	}()
}

const (
	buy    = "buy"
	sell   = "sell"
	header = `
| Action | Symbol | Amount |  Tx   |
|:-------|:-------|:-------|:------|
`
	// content = `|%-2s  |%-2s  |%-2s  | <a href='https://etherscan.io/tx/%s'>tx</a> |`
	content = `|%-2s  |%-2s  |%-2s  | [tx](https://etherscan.io/tx/%s) |`
)

func constructWithTx(txs []ethscan.Tx, addr, alias string) string {
	var hl string
	tx := txs[0]
	if tx.From == addr {
		// create or selling
		hl = strings.Repeat(":heart_decoration:", 5)
	}
	var sb bytes.Buffer
	sb.WriteString(fmt.Sprintf("%s addr: %s last (%d) txs:", hl, alias, len(txs)))
	sb.WriteString("\n")
	sb.WriteString(header)
	n := len(txs)
	for i, tx := range txs {
		if tx.From == addr {
			tx.Action = sell
		} else {
			tx.Action = buy
		}
		td, _ := strconv.Atoi(tx.TokenDecimal)
		na := len(tx.Value) - td
		if na < 0 {
			log.Infof("token: %s with decimal:%s value: %s is wrong.", tx.TokenName, tx.TokenDecimal, tx.Value)
			na = len(tx.Value)
		}
		sb.WriteString(fmt.Sprintf(content, tx.Action, tx.TokenSymbol, tx.Value[:na], tx.Hash))
		if i < n-1 {
			sb.WriteByte('\n')
		}
	}
	// sb.WriteString("</pre>")
	return sb.String()
}
