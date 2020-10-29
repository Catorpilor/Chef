package main

import (
	"context"
	"flag"
	"fmt"
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
				case "set":
					log.Infof("set command message context: %s", update.Message.CommandArguments())
					rawargs := update.Message.CommandArguments()
					args := strings.Fields(rawargs)
					if len(args) != 2 {
						msg.Text = "invalid arguments wanted two."
						break
					}
					if err := watcher.Set(args[0], args[1]); err != nil {
						msg.Text = err.Error()
					}
				case "add":
					log.Infof("add comand message context: %s", update.Message.CommandArguments())
					rawargs := update.Message.CommandArguments()
					args := strings.Fields(rawargs)
					if len(args) < 1 || len(args) > 2 {
						msg.Text = "invalid argumenets, wanted one"
						break
					}
					alias := "default"
					if len(args) > 1 {
						alias = args[1]
					}
					watcher.Add(args[0], update.Message.Chat.ID, alias)
					msg.Text = "add address to the watch list."
				case "delete":
					// this is ugly
					// TODO()
					log.Infof("delete command message context: %s", update.Message.CommandArguments())
					rawargs := update.Message.CommandArguments()
					args := strings.Fields(rawargs)
					if len(args) != 1 {
						msg.Text = "invalid arguments, only allowed one"
						break
					}
					watcher.Delete(args[0])
					msg.Text = fmt.Sprintf("delete %s from the watch list", args[0])
				default:
					msg.Text = "not supported"
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
