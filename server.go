package main

import (
	"bufio"
	"crypto/tls"
	"errors"
	"flag"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"

	"github.com/empirefox/gotool/paas"
	"github.com/golang/glog"
	"github.com/gorilla/websocket"
)

const (
	pingPeriod         = 45 * time.Second
	bufSize            = 32 << 10
	h2DataFrameBufSize = 32 << 10
)

var (
	httpServer *http.Server
	localAddr  net.Addr

	upgrader = websocket.Upgrader{
		ReadBufferSize:  bufSize,
		WriteBufferSize: bufSize,
	}

	h2sleep   time.Duration = 1
	h2sleepup time.Duration = 30

	ErrGlobalWsListenerClosed = errors.New("globalWsListener closed")
)

func init() {
	httpServer = newHttpServer()
}

func main() {
	flag.Parse()
	http2.VerboseLogs = false

	go serveH2()
	glog.Fatal(httpServer.ListenAndServe())
}

func serveH2() {
	tlsConfig := newTlsConfig()
	tlsListener := tls.NewListener(globalWsListener{}, tlsConfig)
	h2Server := newH2Server()
	for {
		glog.Errorln(h2Server.Serve(tlsListener))
		time.Sleep(time.Second * h2sleep)
		if h2sleep < h2sleepup {
			h2sleep++
		}
	}
}

func servHa(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
}

var globalWsChan = make(chan *Ws)

type globalWsListener struct {
}

func (globalWsListener) Accept() (c net.Conn, err error) {
	if ws, ok := <-globalWsChan; ok {
		h2sleep = 1
		if localAddr == nil {
			localAddr = ws.LocalAddr()
		}
		return ws, nil
	}
	return nil, ErrGlobalWsListenerClosed
}

func (globalWsListener) Addr() net.Addr {
	return localAddr
}

func (globalWsListener) Close() error {
	glog.Infoln("globalWsListener.Close called")
	return nil
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

func newH2Server() *http.Server {
	wsMux := http.NewServeMux()
	wsMux.HandleFunc("/r", serveH2r)
	wsMux.HandleFunc("/c", serveH2c)
	wsServer := &http.Server{Addr: paas.BindAddr, Handler: wsMux}
	http2.ConfigureServer(wsServer, nil)
	return wsServer
}

func serveH2c(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			glog.Errorln(err)
		}
	}()
	glog.Infoln("CONNECT to", r.Host)
	remote, err := net.DialTimeout("tcp", r.Host, time.Second*10)
	if err != nil {
		glog.Errorln(err)
		w.WriteHeader(http.StatusNotImplemented)
		return
	}
	defer remote.Close()

	fw := &flushWriter{w}
	fw.FlushHeader(http.StatusOK)
	go io.Copy(remote, r.Body)
	srcRemote := &TryReader{
		c:        remote,
		ignore:   3,
		maxRetry: 2,
		tryDur:   time.Millisecond * 600,
		timeout:  time.Second * 15,
	}
	io.Copy(fw, srcRemote)
}

type TryReader struct {
	c        net.Conn
	ignore   int // for tls ahndshake
	ignored  int
	maxRetry int
	tryDur   time.Duration
	timeout  time.Duration
}

func (r *TryReader) Read(buf []byte) (n int, err error) {
	if r.ignored < r.ignore {
		r.ignored++
		return r.c.Read(buf)
	}
	lb := len(buf)
	var try int
	for n < lb && err == nil && try < r.maxRetry {
		try++
		r.c.SetReadDeadline(time.Now().Add(r.tryDur))
		var nn int
		nn, err = r.c.Read(buf[n:])
		n += nn
	}
	if n == lb {
		err = nil
	} else if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
		if n > 0 {
			err = nil
		} else {
			r.c.SetReadDeadline(time.Now().Add(r.timeout))
			n, err = r.c.Read(buf)
		}
	}
	return
}

func serveH2r(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			glog.Errorln(err)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}()

	if r.Host == "i:80" {
		w.WriteHeader(http.StatusFound)
		return
	}
	glog.Infoln("REVERSE to", r.Host)
	remote, err := net.DialTimeout("tcp", r.Host, time.Second*10)
	if err != nil {
		glog.Errorln(err)
		w.WriteHeader(http.StatusNotImplemented)
		return
	}
	defer remote.Close()

	go io.Copy(remote, r.Body)
	//	go io.Copy(remote, io.TeeReader(r.Body, os.Stdout))
	resr := io.TeeReader(remote, w)
	//	resr = io.TeeReader(resr, os.Stdout)
	res, err := http.ReadResponse(bufio.NewReader(resr), nil)
	if err != nil {
		glog.Errorln(err)
		return
	}
	if res.Body != nil {
		defer res.Body.Close()
		io.Copy(ioutil.Discard, res.Body)
	}
}

type flushWriter struct {
	http.ResponseWriter
}

func (w *flushWriter) FlushHeader(code int) {
	w.ResponseWriter.WriteHeader(code)
	w.ResponseWriter.(http.Flusher).Flush()
}

func (w *flushWriter) Write(p []byte) (n int, err error) {
	n, err = w.ResponseWriter.Write(p)
	if n > 0 {
		w.ResponseWriter.(http.Flusher).Flush()
	}
	return
}

func newTlsConfig() *tls.Config {
	cert, err := tls.LoadX509KeyPair("server.crt", "server.key")
	if err != nil {
		glog.Fatal(err)
	}

	config := tls.Config{
		ClientAuth:   tls.NoClientCert,
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		NextProtos:   []string{http2.NextProtoTLS},
	}
	return &config
}
