package config

import (
	"time"

	"github.com/uber-go/zap"
)

type Config struct {
	WsBufSizeKB int    `default:"65" validate:"gt=0"`
	H2BufSizeKB uint32 `default:"64" validate:"gt=0"`

	H2Logs             bool          `                env:"H2_LOGS"`
	H2RetryMaxSecond   time.Duration `default:"30ns"  env:"H2_RETRY_MAX_SECOND"`
	H2SleepToRunSecond time.Duration `default:"2ns"   env:"H2_SLEEP_SECOND"`
	PingSecond         uint          `default:"45"    env:"PING_SECOND"`
	TCP                uint64        `                env:"WSH_TCP"`
	ZapLevel           string        `default:"error" env:"ZAP_LEVEL" validate:"zap_level"`

	ServerCrt []byte `json:"-" validate:"gt=0" xps:"server.crt"`
	ServerKey []byte `json:"-" validate:"gt=0" xps:"server.key"`
	ChainPerm []byte `json:"-" validate:"gt=0" xps:"chain.pem"`
	BricksPac []byte `json:"-" validate:"gt=0" xps:"bricks.pac"`

	Env    Env        `json:"-"`
	Logger zap.Logger `json:"-"`
}
