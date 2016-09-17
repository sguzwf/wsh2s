package wsh2s

import "net/http"

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
