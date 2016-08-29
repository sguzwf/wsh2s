package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	"golang.org/x/net/http2"

	"github.com/Sirupsen/logrus"
	"github.com/empirefox/gotool/paas"
	"github.com/gorilla/websocket"
)

const (
	pingPeriod = 45 * time.Second
	bufSize    = 32 << 10
)

var (
	h2v = flag.Bool("h2v", false, "enable http2 verbose logs")

	log  = logrus.New()
	logf = log.Printf

	challengeProvider *wrapperChallengeProvider
	httpServer        *http.Server

	upgrader = websocket.Upgrader{
		ReadBufferSize:  bufSize,
		WriteBufferSize: bufSize,
	}

	pacResponseBytes []byte
)

func init() {
	// Log as JSON instead of the default ASCII formatter.
	logrus.SetFormatter(&logrus.TextFormatter{})

	// Output to stderr instead of stdout, could also be a file.
	logrus.SetOutput(os.Stderr)

	// Only log the warning severity or above.
	logrus.SetLevel(logrus.WarnLevel)

	challengeProvider = new(wrapperChallengeProvider)

	httpServer = newHttpServer()
}

// ACME_DOMAINS=www.xxx.com
func main() {
	flag.Parse()
	http2.VerboseLogs = *h2v

	if ps, err := ioutil.ReadFile("bricks.pac"); err != nil {
		log.Fatal(err)
	} else {
		var b bytes.Buffer
		b.WriteString("HTTP/1.1 200 OK\r\nContent-Length: ")
		b.WriteString(strconv.Itoa(len(ps)))
		b.WriteString("\r\n\r\n")
		b.Write(ps)
		pacResponseBytes = b.Bytes()
	}

	go serveH2()
	log.Fatal(httpServer.ListenAndServe())
}

func serveHa(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func serveWs(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Errorln(err)
		}
	}()
	log.Infoln("new websocket request")
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorln(err)
		return
	}
	log.Infoln("websocket ok")
	c := NewWs(ws, bufSize, pingPeriod)
	globalWsChan <- c
}

func newHttpServer() *http.Server {
	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/h2p", serveWs)
	httpMux.HandleFunc("/", serveHa)
	httpMux.HandleFunc("/.well-known/acme-challenge/", challengeProvider.challengeHanlder)
	httpServer := &http.Server{Addr: paas.BindAddr, Handler: httpMux}
	return httpServer
}
