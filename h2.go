package wsh2s

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/uber-go/zap"
	"golang.org/x/net/http2"
)

func (s *Server) listenAndServeH2All() {
	time.Sleep(time.Second * s.config.H2SleepToRunSecond)

	tlsConfig, err := s.newH2TlsConfig()
	if err != nil {
		return
	}

	s.listenAndServeH2(tlsConfig, s.config.TCP != 0)
}

func (s *Server) listenAndServeH2(tlsConfig *tls.Config, tcp bool) {
	var laddr string
	var tlsListener net.Listener
	var err error
	if tcp {
		if s.config.TCP == 0 {
			return
		}
		laddr = fmt.Sprintf(":%d", s.config.TCP)
		tlsListener, err = tls.Listen("tcp", laddr, tlsConfig)
		if err != nil {
			s.logger.Error("tcp tlsListener failed", zap.Error(err))
			return
		}
	} else {
		laddr = ":8444"
		tlsListener = tls.NewListener(newGlobalWsListener(s.globalWsChan), tlsConfig)
	}

	h2Server, afterServeError := s.newH2Server(tlsConfig, laddr)
	for {
		err = h2Server.Serve(tlsListener)
		afterServeError(err)
	}
}

func (s *Server) newH2Server(tlsConfig *tls.Config, laddr string) (*http.Server, func(error)) {
	var mu sync.Mutex
	var h2sleep time.Duration = 1
	h2Server := &http.Server{
		Addr:      laddr,
		Handler:   http.HandlerFunc(s.serveH2),
		TLSConfig: tlsConfig,
		ConnState: func(c net.Conn, s http.ConnState) {
			if s == http.StateNew {
				mu.Lock()
				h2sleep = 1
				mu.Unlock()
			}
		},
	}
	http2.ConfigureServer(h2Server, &http2.Server{
		MaxReadFrameSize: s.config.H2BufSizeKB << 10,
	})

	afterServeError := func(err error) {
		s.logger.Error("h2 server failed", zap.Error(err))
		mu.Lock()
		if h2sleep < s.config.H2RetryMaxSecond {
			h2sleep++
		}
		sec := h2sleep
		mu.Unlock()
		time.Sleep(time.Second * sec)
	}
	return h2Server, afterServeError
}

func (s *Server) serveH2(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "CONNECT":
		s.serveH2c(w, r)
	case r.Host == "i:80":
		w.WriteHeader(http.StatusOK)
	case r.Host == "i:81":
		w.Write(s.info)
	case r.Host == "i:82":
		w.Write(s.config.BricksPac)
	case r.URL.Path == "/r" && r.Method == "POST":
		s.serveH2r(w, r)
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

func (s *Server) serveH2c(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			s.logger.Error("CONNECT failed", zap.Object("err", err))
		}
	}()
	remote, err := net.DialTimeout("tcp", r.Host, time.Second*15)
	if err != nil {
		s.logger.Error("dail failed", zap.Error(err), zap.String("host", r.Host))
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

func (s *Server) serveH2r(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			s.logger.Error("REVERSE failed", zap.Object("err", err))
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}()

	remote, err := net.DialTimeout("tcp", r.Host, time.Second*15)
	if err != nil {
		s.logger.Error("dail failed", zap.Error(err), zap.String("host", r.Host))
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
		return
	}
	if res.Body != nil {
		defer res.Body.Close()
		io.Copy(ioutil.Discard, res.Body)
	}
}

func (s *Server) newH2TlsConfig() (*tls.Config, error) {
	// 1. LoadServerCert
	cert, err := tls.X509KeyPair(s.config.ServerCrt, s.config.ServerKey)
	if err != nil {
		s.logger.Error("loading server certificate", zap.Error(err))
		return nil, err
	}

	// 2. LoadCACert
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(s.config.ChainPerm) {
		return nil, errors.New("loading CA certificate failed")
	}

	config := tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
		NextProtos:   []string{http2.NextProtoTLS},
	}
	return &config, nil
}
