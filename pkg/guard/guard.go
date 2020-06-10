package guard

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/catorpilor/idenaMgrBot/idena"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/gomodule/redigo/redis"
	log "github.com/sirupsen/logrus"
)

const (
	defaultInterval = 15 * time.Minute
	prefixLen       = 11 // idena:addr: length
)

type Guardian struct {
	bot          *tgbotapi.BotAPI
	watched      map[string]int64
	lastActivity map[string]int64
	ticker       *time.Ticker
	idenaCtl     *idena.Client
	redisCtl     *redis.Pool
	lock         sync.RWMutex
}

func New(bot *tgbotapi.BotAPI, ctl *idena.Client, redisCtl *redis.Pool) *Guardian {
	guard := &Guardian{
		bot:          bot,
		watched:      make(map[string]int64),
		lastActivity: make(map[string]int64),
		ticker:       time.NewTicker(defaultInterval),
		idenaCtl:     ctl,
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
	keys, err := redis.Strings(c.Do("KEYS", "idena:addr:*"))
	if err != nil {
		log.Infof("get keys idena:addr:* failed with err:%s", err.Error())
		return
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
	guard.lock.Lock()
	for i := range keys {
		guard.watched[keys[i][prefixLen:]] = vals[i]
	}
	guard.lock.Unlock()
	log.Infof("udpate watched list:%v", guard.watched)
}

func (guard *Guardian) isWatched(addr string) bool {
	guard.lock.RLock()
	defer guard.lock.RUnlock()
	if _, exists := guard.watched[addr]; exists {
		return true
	}
	return false
}

func (guard *Guardian) Add(addr string, chatID int64) {
	if guard.isWatched(addr) {
		return
	}
	c := guard.redisCtl.Get()
	defer c.Close()
	_ = c.Send("MULTI")
	_ = c.Send("INCR", "idena:count")
	_ = c.Send("SET", fmt.Sprintf("idena:addr:%s", addr), strconv.FormatInt(chatID, 10))
	if _, err := c.Do("EXEC"); err != nil {
		log.Infof("persist addr:%s and chatID:%d got err:%s", addr, chatID, err.Error())
		// TODO(@catorpilor): add retry logic here.
	}
	guard.lock.Lock()
	guard.watched[addr] = chatID
	guard.lock.Unlock()
}

func (guard *Guardian) Start() {
	go func() {
		for {
			select {
			case <-guard.ticker.C:
				for addr, chatID := range guard.watched {
					go func(addr string, chatID int64) {
						offLine := false
						os := guard.idenaCtl.CheckOnlineIdentity(addr)
						if os == nil {
							log.Info("calling check status got nil")
							return
						}
						log.Infof("calling addr:%s got %v", addr, os)
						if v, exists := guard.lastActivity[addr]; exists {
							offLine = (v == os.LastTime)
						}
						if offLine || !os.Online { //
							msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("addr:%s is offline and lastActivity is %s",
								addr, os.LastActivity))
							// send message
							_, _ = guard.bot.Send(msg)
						}
						guard.lastActivity[addr] = os.LastTime
					}(addr, chatID)
				}
			}
		}

	}()
}
