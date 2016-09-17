package wsh2s

import (
	"errors"
	"net"
	"time"
)

var (
	ErrGlobalWsListenerClosed = errors.New("globalWsListener closed")
)

type globalWsListener struct {
	globalWsChan <-chan *Ws
	localAddr    net.Addr
	h2sleep      time.Duration
}

func newGlobalWsListener(globalWsChan <-chan *Ws) *globalWsListener {
	return &globalWsListener{globalWsChan: globalWsChan, h2sleep: 1}
}

func (l *globalWsListener) Accept() (c net.Conn, err error) {
	if ws, ok := <-l.globalWsChan; ok {
		l.h2sleep = 1
		if l.localAddr == nil {
			l.localAddr = ws.LocalAddr()
		}
		return ws, nil
	}
	return nil, ErrGlobalWsListenerClosed
}

func (l *globalWsListener) Addr() net.Addr {
	return l.localAddr
}

func (l *globalWsListener) Close() error {
	return nil
}
