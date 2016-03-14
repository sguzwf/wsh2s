package main

import (
	"net"
	"time"
)

type TryReader struct {
	c        net.Conn
	ignore   int // for tls ahndshake, normally set 3
	ignored  int // a counter
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
