package wsh2s

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/empirefox/acmewrapper"
	"github.com/uber-go/zap"
	"golang.org/x/net/http2"
)

func (s *Server) listenAndServeH2All() {
	time.Sleep(time.Second * s.H2SleepToRunSecond)

	tlsConfig, err := s.newH2TlsConfig()
	if err != nil {
		return
	}

	go s.listenAndServeH2(tlsConfig, true)
	s.listenAndServeH2(tlsConfig, false)
}

func (s *Server) listenAndServeH2(tlsConfig *tls.Config, tcp bool) {
	var laddr string
	var tlsListener net.Listener
	var err error
	if tcp {
		if s.TCP == 0 {
			return
		}
		laddr = fmt.Sprintf(":%d", s.TCP)
		tlsListener, err = tls.Listen("tcp", laddr, tlsConfig)
		if err != nil {
			Log.Error("tcp tlsListener failed", zap.Error(err))
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
		MaxReadFrameSize: s.H2BufSize,
	})

	afterServeError := func(err error) {
		Log.Error("h2 server failed", zap.Error(err))
		mu.Lock()
		if h2sleep < s.H2RetryMaxSecond {
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
		w.Write(s.getPac())
	case r.Host == "i:83":
		w.Write(s.tryLoadPac())
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
			Log.Error("CONNECT failed", zap.Object("err", err))
		}
	}()
	remote, err := net.DialTimeout("tcp", r.Host, time.Second*10)
	if err != nil {
		Log.Error("dail failed", zap.Error(err), zap.String("host", r.Host))
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
			Log.Error("REVERSE failed", zap.Object("err", err))
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}()

	remote, err := net.DialTimeout("tcp", r.Host, time.Second*10)
	if err != nil {
		Log.Error("dail failed", zap.Error(err), zap.String("host", r.Host))
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
	if os.Getenv("TEST_MODE") == "1" {
		cert, err := tls.LoadX509KeyPair("server.crt", "server.key")
		if err != nil {
			Log.Error("load keys failed", zap.Error(err))
			return nil, err
		}

		config := tls.Config{
			ClientAuth:   tls.NoClientCert,
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
			NextProtos:   []string{http2.NextProtoTLS},
		}
		return &config, nil
	}

	w, err := acmewrapper.New(acmewrapper.Config{
		Domains: []string{s.AcmeDomain},

		TLSCertFile: fmt.Sprintf("/%s/%s", s.AcmeDomain, "cert.pem"),
		TLSKeyFile:  fmt.Sprintf("/%s/%s", s.AcmeDomain, "key.pem"),

		RegistrationFile: fmt.Sprintf("/%s/%s", s.AcmeDomain, "user.reg"),
		PrivateKeyFile:   fmt.Sprintf("/%s/%s", s.AcmeDomain, "user.pem"),

		TOSCallback: acmewrapper.TOSAgree,

		HTTP01ChallengeProvider: s.challengeProvider,

		SaveFileCallback: s.dbox.SaveFile,
		LoadFileCallback: s.dbox.LoadFile,
	})
	if err != nil {
		Log.Error("acmewrapper failed", zap.Error(err))
		return nil, err
	}
	return w.TLSConfig(), nil
}
