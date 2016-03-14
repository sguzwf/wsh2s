package main

import (
	"flag"
	"net/http"
	"time"

	"golang.org/x/net/http2"

	"github.com/empirefox/gotool/paas"
	"github.com/golang/glog"
	"github.com/gorilla/websocket"
)

const (
	pingPeriod = 45 * time.Second
	bufSize    = 32 << 10
)

var (
	h2v = flag.Bool("h2v", false, "enable http2 verbose logs")

	httpServer *http.Server

	upgrader = websocket.Upgrader{
		ReadBufferSize:  bufSize,
		WriteBufferSize: bufSize,
	}
)

func init() {
	httpServer = newHttpServer()
}

func main() {
	flag.Parse()
	http2.VerboseLogs = *h2v

	go serveH2()
	glog.Fatal(httpServer.ListenAndServe())
}

func servHa(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func serveWs(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			glog.Errorln(err)
		}
	}()
	glog.Infoln("new websocket request")
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		glog.Errorln(err)
		return
	}
	glog.Infoln("websocket ok")
	c := NewWs(ws, bufSize, pingPeriod)
	globalWsChan <- c
}

func newHttpServer() *http.Server {
	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/h2p", serveWs)
	httpMux.HandleFunc("/", servHa)
	httpServer := &http.Server{Addr: paas.BindAddr, Handler: httpMux}
	return httpServer
}
