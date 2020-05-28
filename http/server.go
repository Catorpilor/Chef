package http

import (
	"encoding/json"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

const (
	readTimeOut  = 30 * time.Second
	writeTimeOut = 40 * time.Second
	okStr        = "OK"
)

func NewServer(addr string, mds ...mux.MiddlewareFunc) *http.Server {
	router := mux.NewRouter()
	for _, md := range mds {
		router.Use(md)
	}
	router.HandleFunc("/debug/pprof/", pprof.Index)
	router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	router.HandleFunc("/debug/pprof/profile", pprof.Profile)
	router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	router.HandleFunc("/debug/pprof/trace", pprof.Trace)
	router.Handle("/debug/pprof/block", pprof.Handler("block"))
	router.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	router.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	router.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	router.Handle("/metrics", promhttp.Handler())
	router.HandleFunc("/healthz", healthCheck)
	return &http.Server{Addr: addr, Handler: router, ReadTimeout: readTimeOut, WriteTimeout: writeTimeOut}
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	responseWithText(w, okStr)
}

type mmResp struct {
	// ResultCode        int    `json:"resultCode"`
	// ResultDescription string `json:"resultDescription"`
	// Message           string `json:"message,omitempty"`
	Text string `json:"text,omitempty"`
}

func responseWithText(w http.ResponseWriter, text string) {
	st := mmResp{Text: text}
	b, err := json.Marshal(st)
	if err != nil {
		log.Infof("json marshal %v got err: %s\n", st, err.Error())
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}
