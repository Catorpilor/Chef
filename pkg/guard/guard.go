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
	bot *tgbotapi.BotAPI
	// watched is the local cache that key is the address, values are chatids.
	watched map[string][]int64
	// lastActivity store the address's latest block.
	lastActivity map[string]int64
	// alias store the alias for the address, key is "chatid-address"
	alias    map[string]string
	ticker   *time.Ticker
	ethCtl   *ethscan.Client
	redisCtl *redis.Pool
	lock     sync.RWMutex
}

func New(bot *tgbotapi.BotAPI, ctl *ethscan.Client, redisCtl *redis.Pool, interval int) *Guardian {
	guard := &Guardian{
		bot:          bot,
		watched:      make(map[string][]int64),
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
	guard.lock.Lock()
	defer guard.lock.Unlock()
	count, err := redis.Int(c.Do("GET", "idena:count"))
	if err != nil {
		if errors.Is(err, redis.ErrNil) {
			return
		}
		log.Infof("get key:idena:count from redis got err:%s", err.Error())
		return
	}
	log.Infof("get address count: %d", count)
	if count < 1 {
		return
	}
	keys, err := redis.Strings(c.Do("KEYS", "user:*"))
	if err != nil {
		log.Infof("get keys users:* failed with err:%s", err.Error())
		return
	}
	for _, key := range keys {
		// idx := strings.LastIndex(key, ":")
		user := key[5:]
		userID, err := strconv.ParseInt(user, 10, 64)
		if err != nil {
			log.Errorf("parse %s failed got err: %v", user, err)
			continue
		}
		addrs, err := redis.Strings(c.Do("SMEMBERS", key))
		if err != nil {
			log.Errorf("smembers %s failed %v", key, err)
			continue
		}
		log.Infof("get addrs %v from user:%s", addrs, user)
		_ = c.Send("MULTI")
		for _, addr := range addrs {
			_ = c.Send("GET", fmt.Sprintf("ether:addr:%s:%s:alias", user, addr))
			guard.watched[addr] = append(guard.watched[addr], userID)
		}
		vals, err := redis.Strings(c.Do("EXEC"))
		for i, val := range vals {
			guard.alias[user+"-"+addrs[i]] = val
			log.Infof("get alias %s for user: %s with address: %s", val, user, addrs[i])
		}
	}
	// keys, err := redis.Strings(c.Do("KEYS", "ether:addr:*"))
	// if err != nil {
	// 	log.Infof("get keys idena:addr:* failed with err:%s", err.Error())
	// 	return
	// }
	activityKeys, err := redis.Strings(c.Do("KEYS", "lastActivity:*"))
	if err != nil {
		log.Infof("get activityKeys got err:%s", err)
	}
	// _ = c.Send("MULTI")
	// for _, key := range keys {
	// 	_ = c.Send("GET", key)
	// }
	// vals, err := redis.Int64s(c.Do("EXEC"))
	// if err != nil {
	// 	log.Infof("exec get vals from keys:%v failed %s", keys, err.Error())
	// 	return
	// }
	_ = c.Send("MULTI")
	for _, key := range activityKeys {
		_ = c.Send("GET", key)
	}
	activities, err := redis.Strings(c.Do("EXEC"))
	if err != nil {
		log.Infof("multi/exec activityKeys:%v got err:%v", activityKeys, err)
	}
	// // get alias back
	// _ = c.Send("MULTI")
	// for _, key := range keys {
	// 	_ = c.Send("GET", fmt.Sprintf("addr:%s:alias", key[prefixLen:]))
	// }
	// alias, err := redis.Strings(c.Do("EXEC"))
	// if err != nil {
	// 	log.Infof("multi/exec alias got err: %v", err)
	// }
	// guard.lock.Lock()
	// for i := range keys {
	// 	guard.watched[keys[i][prefixLen:]] = vals[i]
	// }
	for i := range activityKeys {
		bn, err := strconv.ParseInt(activities[i], 10, 64)
		if err != nil {
			log.Infof("parseInt with arg: %s got err:%v", activities[i], err)
			bn = 0
		}
		lastIdx := strings.LastIndex(activityKeys[i], ":")
		addr := activityKeys[i][lastIdx+1:]
		key := activityKeys[i][13:lastIdx] + "-" + addr
		log.Infof("update key: %s with bn:%d", key, bn)
		guard.lastActivity[key] = bn
	}
	log.Infof("udpate watched list:%v, and activityList:%v", guard.watched, guard.lastActivity)
}

func (guard *Guardian) Set(addr, alia string, chatID int64) error {
	if !guard.isWatched(addr, chatID) {
		return unknownAddrErr
	}
	guard.lock.Lock()
	defer guard.lock.Unlock()
	guard.alias[fmt.Sprintf("%d-%s", chatID, addr)] = alia
	c := guard.redisCtl.Get()
	defer c.Close()
	_, err := c.Do("SET", fmt.Sprintf("ether:addr:%d:%s:alias", chatID, addr), alia)
	return err
}

func (guard *Guardian) isWatched(addr string, chatID int64) bool {
	guard.lock.RLock()
	defer guard.lock.RUnlock()
	if _, exists := guard.alias[fmt.Sprintf("%d-%s", chatID, addr)]; exists {
		return true
	}
	return false
}

func (guard *Guardian) Add(addr string, chatID int64, alias string) {
	if guard.isWatched(addr, chatID) {
		log.Infof("user: %d already watched address:%s", chatID, addr)
		return
	}
	c := guard.redisCtl.Get()
	defer c.Close()
	_ = c.Send("MULTI")
	_ = c.Send("INCR", "idena:count")
	_ = c.Send("SADD", fmt.Sprintf("user:%d", chatID), addr)
	_ = c.Send("SET", fmt.Sprintf("ether:addr:%d:%s:alias", chatID, addr), alias)
	if _, err := c.Do("EXEC"); err != nil {
		log.Infof("persist addr:%s and chatID:%d got err:%s", addr, chatID, err.Error())
		// TODO(@catorpilor): add retry logic here.
	}
	guard.lock.Lock()
	guard.watched[addr] = append(guard.watched[addr], chatID)
	guard.alias[fmt.Sprintf("%d-%s", chatID, addr)] = alias
	guard.lock.Unlock()
}

func (guard *Guardian) Delete(addr string, chatID int64) error {
	if !guard.isWatched(addr, chatID) {
		return errors.New("user not add this address")
	}
	guard.lock.Lock()
	chatIDs := guard.watched[addr]
	var idx int
	n := len(chatIDs)
	for idx < n {
		if chatIDs[idx] == chatID {
			break
		}
		idx++
	}
	chatIDs[idx] = chatIDs[n-1]
	chatIDs = chatIDs[:n-1]
	guard.watched[addr] = chatIDs
	delete(guard.lastActivity, fmt.Sprintf("%d-%s", chatID, addr))
	if n == 1 {
		delete(guard.watched, addr)
	}

	guard.lock.Unlock()
	c := guard.redisCtl.Get()
	defer c.Close()
	_ = c.Send("MULTI")
	_ = c.Send("DECR", "idena:count")
	_ = c.Send("SREM", fmt.Sprintf("user:%d", chatID), addr)
	_ = c.Send("DEL", fmt.Sprintf("ether:addr:%d:%s:alias", chatID, addr))
	_ = c.Send("DEL", fmt.Sprintf("lastActivity:%d:%s", chatID, addr))
	if _, err := c.Do("EXEC"); err != nil {
		log.Infof("delete addr:%s in redis got err:%v", addr, err)
		return err
	}
	return nil
}

func (guard *Guardian) Start() {
	go func() {
		for {
			select {
			case <-guard.ticker.C:
				for addr, chatIDs := range guard.watched {
					go func(addr string, chatIDs []int64) {
						c := guard.redisCtl.Get()
						defer c.Close()
						// no need to loop call, just pick the oldest block.
						// afterall they all point to the same block.
						var startBlock string
						lk := fmt.Sprintf("%d-%s", chatIDs[0], addr)
						if guard.lastActivity[lk] != 0 {
							startBlock = strconv.FormatInt(guard.lastActivity[lk], 10)
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
								for _, chatID := range chatIDs {
									guard.lastActivity[lk] = bn + 1
									if _, err := c.Do("SET", fmt.Sprintf("lastActivity:%d:%s", chatID, addr), rp.Result[0].BlockNumber); err != nil {
										log.Infof("set lastActivity:%s got err:%v", addr, err)
									}
									msg := tgbotapi.NewMessage(chatID, constructWithTx(rp.Result, addr, guard.alias[lk]))
									msg.ParseMode = tgbotapi.ModeMarkdown
									_, err = guard.bot.Send(msg)
									if err != nil {
										log.Infof("send message: %v  to chatID: %d got  err:%v", msg, chatID, err)
										continue
									}
								}
							}
						}
					}(addr, chatIDs)
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
