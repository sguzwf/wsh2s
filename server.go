package wsh2s

import (
	"encoding/json"
	"errors"
	"net/http"

	"golang.org/x/net/http2"

	"github.com/empirefox/cement/clog"
	"github.com/empirefox/gotool/paas"
	"github.com/empirefox/wsh2s/config"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type Server struct {
	config config.Server

	logger clog.Logger

	httpServer *http.Server
	upgrader   websocket.Upgrader

	// globalWsListener
	globalWsChan chan *Ws
	info         []byte
}

func NewServer(config *config.Config) (*Server, error) {
	http2.VerboseLogs = config.Server.H2Logs

	logger, err := clog.NewLogger(config.Clog)
	if err != nil {
		logger.Error("init clog failed", zap.Error(err))
		return nil, err
	}

	s := &Server{
		config:       config.Server,
		logger:       logger,
		globalWsChan: make(chan *Ws),
	}

	s.upgrader.ReadBufferSize = s.config.WsBufSizeKB << 10
	s.upgrader.WriteBufferSize = s.config.WsBufSizeKB << 10

	info, err := json.Marshal(map[string]interface{}{
		"PingSecond": s.config.PingSecond,
	})
	if err != nil {
		logger.Error("compute server info", zap.Error(err))
		return nil, err
	}

	s.info = info

	return s, nil
}

func (s *Server) Serve() error {

	if s.config.TCP == 0 {
		s.httpServer = s.newHttpServer()

		go s.listenAndServeH2All()
		return s.httpServer.ListenAndServe()
	}

	s.listenAndServeH2All()
	return errors.New("TCP server failed")
}

func (s *Server) serveWs(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			s.logger.Error("serveWs error", zap.Any("err", err))
		}
	}()
	s.logger.Debug("websocket start")
	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		s.logger.Error("websocket failed", zap.Error(err))
		return
	}
	s.logger.Debug("websocket ok")
	s.globalWsChan <- NewWs(ws, s.config.WsBufSizeKB<<10)
}

func (s *Server) newHttpServer() *http.Server {
	httpMux := http.NewServeMux()

	httpMux.HandleFunc("/p", s.serveWs)
	httpMux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return &http.Server{Addr: paas.BindAddr, Handler: httpMux}
}
