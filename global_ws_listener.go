package main

import (
	"errors"
	"net"

	"github.com/golang/glog"
)

var (
	localAddr net.Addr

	globalWsChan = make(chan *Ws)

	ErrGlobalWsListenerClosed = errors.New("globalWsListener closed")
)

type globalWsListener struct{}

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