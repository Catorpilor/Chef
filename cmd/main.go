package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"github.com/catorpilor/ethChef"
	"github.com/catorpilor/ethChef/ethscan"
	localhttp "github.com/catorpilor/ethChef/http"
	"github.com/catorpilor/ethChef/pkg/guard"
	"github.com/catorpilor/ethChef/redis"
	"github.com/catorpilor/ethChef/telegram"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	log "github.com/sirupsen/logrus"
)

const (
	defaultOffset = 0
)

func main() {
	f := flag.String("toml", "ethchef.toml", "bot's configuration")
	var err error
	var conf ethChef.Config
	if err = ethChef.Load(*f, &conf); err != nil {
		log.Fatalf("load %s got err:%s", *f, err.Error())
	}
	bot, err := telegram.New(conf.Telegram.Token, false)
	if err != nil {
		log.Fatal(err)
	}
	uc := telegram.UpdateConfig(defaultOffset, conf.Telegram.Timeout)
	lc := ethscan.NewCtl(nil, conf.Etherscan.Key)
	rc := redis.NewPool(conf.Redis.Addr, conf.Redis.MaxIdle)
	watcher := guard.New(bot, lc, rc, conf.Etherscan.Interval)
	go watcher.Start()
	go func() {
		updates, err := bot.GetUpdatesChan(uc)
		if err != nil {
			log.Infof("getUpdatesChan got with config:%v got err:%s", uc, err.Error())
		}
		for update := range updates {
			if update.Message == nil {
				continue
			}
			log.Infof("got message: [%s] %v", update.Message.From.UserName, update.Message)
			if update.Message.IsCommand() {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
				switch update.Message.Command() {
				case "status", "last":
					msg.Text = "not avaliable yet."
				case "add":
					log.Infof("message context: %s", update.Message.CommandArguments())
					rawargs := update.Message.CommandArguments()
					args := strings.Fields(rawargs)
					if len(args) > 1 || len(args) < 1 {
						msg.Text = "invalid argumenets, wanted one"
						break
					}
					watcher.Add(args[0], update.Message.Chat.ID)
					msg.Text = "add address to the watch list."
				}
				_, err := bot.Send(msg)
				if err != nil {
					log.Infof("send message:%v got err:%s", msg, err.Error())
				}
			}
		}
	}()
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	httpsrv := localhttp.NewServer(conf.Server.Addr)
	go func() {
		<-interrupt
		log.Info("graceful server")
		if err = httpsrv.Shutdown(context.Background()); err != nil {
			log.Infof("could not shutdown: %v", err)
			return
		}
	}()
	log.Infof("starting http server on %s", conf.Server.Addr)
	err = httpsrv.ListenAndServe()
	if err != http.ErrServerClosed {
		log.Fatal(err)
	}

}
