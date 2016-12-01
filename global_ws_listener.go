package wsh2s

import (
	"errors"
	"net"
)

var (
	ErrGlobalWsListenerClosed = errors.New("globalWsListener closed")
)

type globalWsListener struct {
	globalWsChan <-chan *Ws
	localAddr    net.Addr
}

func newGlobalWsListener(globalWsChan <-chan *Ws) *globalWsListener {
	return &globalWsListener{globalWsChan: globalWsChan}
}

func (l *globalWsListener) Accept() (c net.Conn, err error) {
	if ws, ok := <-l.globalWsChan; ok {
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
