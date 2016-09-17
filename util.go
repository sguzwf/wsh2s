package wsh2s

import (
	"bytes"
	"strconv"
)

func wrapToResponse(ps []byte) []byte {
	var b bytes.Buffer
	b.WriteString("HTTP/1.1 200 OK\r\nContent-Length: ")
	b.WriteString(strconv.Itoa(len(ps)))
	b.WriteString("\r\n\r\n")
	b.Write(ps)
	return b.Bytes()
}
