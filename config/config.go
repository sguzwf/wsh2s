package config

import (
	"time"

	"go.uber.org/zap"
)

type Config struct {
	WsBufSizeKB int    `default:"65" validate:"gt=0"`
	H2BufSizeKB uint32 `default:"64" validate:"gt=0"`

	H2Logs             bool          `                env:"H2_LOGS"`
	H2RetryMaxSecond   time.Duration `default:"30ns"  env:"H2_RETRY_MAX_SECOND"`
	H2SleepToRunSecond time.Duration `default:"2ns"   env:"H2_SLEEP_SECOND"`
	PingSecond         int           `default:"45"    env:"PING_SECOND"          validate:"gte=0"`
	TCP                int           `                env:"WSH_TCP"              validate:"gte=0"`
	Dev                bool          `                env:"DEV"`
	ZapLevel           string        `                env:"ZAP_LEVEL"            validate:"zap_level"`

	ServerCrt []byte `json:"-" validate:"gt=0" xps:"server.crt"`
	ServerKey []byte `json:"-" validate:"gt=0" xps:"server.key"`
	ChainPerm []byte `json:"-" validate:"gt=0" xps:"chain.pem"`
	BricksPac []byte `json:"-" validate:"gt=0" xps:"bricks.pac"`

	Env    Env         `json:"-"`
	Logger *zap.Logger `json:"-"`
}
