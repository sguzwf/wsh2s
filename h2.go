package main

import (
	"bufio"
	"crypto/tls"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/empirefox/gotool/paas"
	"github.com/golang/glog"
	"golang.org/x/net/http2"
)

var (
	h2sleep   time.Duration = 1
	h2sleepup time.Duration = 30

	h2DataFrameBufSize = 32 << 10
)

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

func newH2Server() *http.Server {
	http2.VerboseLogs = false
	h2Server := &http.Server{
		Addr:    paas.BindAddr,
		Handler: http.HandlerFunc(servH2),
	}
	http2.ConfigureServer(h2Server, nil)
	return h2Server
}

func servH2(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "CONNECT":
		serveH2c(w, r)
	case r.Host == "i:80":
		w.Write(pacResponseBytes)
	case r.URL.Path == "/r" && r.Method == "POST":
		serveH2r(w, r)
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
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

func serveH2r(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			glog.Errorln(err)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}()

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
