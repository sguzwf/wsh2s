package wsh2s

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/empirefox/gotool/paas"
	"github.com/gorilla/websocket"
	"github.com/uber-go/zap"
)

var (
	Log zap.Logger
)

type Server struct {
	AcmeDomain         string
	DropboxAccessToken string
	DropboxDomainKey   string
	H2RetryMaxSecond   time.Duration
	H2SleepToRunSecond time.Duration
	WsBufSize          int
	PingSecond         uint

	dbox              *dropboxer
	challengeProvider *wrapperChallengeProvider
	httpServer        *http.Server
	upgrader          websocket.Upgrader

	// globalWsListener
	globalWsChan chan *Ws
}

func (s *Server) Serve() error {
	s.globalWsChan = make(chan *Ws)
	if s.WsBufSize == 0 {
		s.WsBufSize = 129 << 10
	}
	if s.PingSecond == 0 {
		s.PingSecond = 45
	}
	s.upgrader.ReadBufferSize = s.WsBufSize
	s.upgrader.WriteBufferSize = s.WsBufSize

	if s.H2RetryMaxSecond == 0 {
		s.H2RetryMaxSecond = 30
	}

	info, err := json.Marshal(map[string]interface{}{
		"PingSecond": s.PingSecond,
		"WsBufSize":  s.WsBufSize,
	})
	if err != nil {
		Log.Error("compute server info", zap.Error(err))
		return err
	}

	s.dbox, err = newDropbox(s.DropboxAccessToken, s.DropboxDomainKey)
	if err != nil {
		Log.Error("create dropbox client", zap.Error(err))
		return err
	}

	ps, err := s.dbox.LoadPlainFile("/bricks.pac")
	if err != nil {
		Log.Error("load pac from dropbox", zap.Error(err))
		return err
	}
	s.httpServer = s.newHttpServer(info, ps)

	s.challengeProvider = new(wrapperChallengeProvider)

	go s.listenAndServeH2()
	return s.httpServer.ListenAndServe()
}

func (s *Server) serveWs(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			Log.Error("serveWs error", zap.Object("err", err))
		}
	}()
	Log.Debug("websocket start")
	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		Log.Error("websocket failed", zap.Error(err))
		return
	}
	Log.Debug("websocket ok")
	s.globalWsChan <- NewWs(ws, s.WsBufSize)
}

func (s *Server) newHttpServer(info, pacResponseBytes []byte) *http.Server {
	httpMux := http.NewServeMux()

	httpMux.HandleFunc("/h2p", s.serveWs)
	httpMux.HandleFunc("/.well-known/acme-challenge/", s.challengeProvider.challengeHanlder)

	httpMux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	httpMux.HandleFunc("/info", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(info)
	})

	httpMux.HandleFunc("/pac", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(pacResponseBytes)
	})

	return &http.Server{Addr: paas.BindAddr, Handler: httpMux}
}
