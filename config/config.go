package config

import (
	"time"

	"github.com/empirefox/cement/clog"
)

type Server struct {
	WsBufSizeKB int    `default:"65" validate:"gt=0"`
	H2BufSizeKB uint32 `default:"64" validate:"gt=0"`

	H2Logs             bool          `                env:"H2_LOGS"`
	H2RetryMaxSecond   time.Duration `default:"30ns"  env:"H2_RETRY_MAX_SECOND"`
	H2SleepToRunSecond time.Duration `default:"2ns"   env:"H2_SLEEP_SECOND"`
	PingSecond         int           `default:"45"    env:"PING_SECOND"          validate:"gte=0"`
	TCP                int           `                env:"WSH_TCP"              validate:"gte=0"`
	Dev                bool          `                env:"DEV"`

	ServerCrt []byte `json:"-" ymal:"-" toml:"-" validate:"gt=0" xps:"server.crt"`
	ServerKey []byte `json:"-" ymal:"-" toml:"-" validate:"gt=0" xps:"server.key"`
	ChainPerm []byte `json:"-" ymal:"-" toml:"-" validate:"gt=0" xps:"chain.pem"`
	BricksPac []byte `json:"-" ymal:"-" toml:"-" validate:"gt=0" xps:"bricks.pac"`
}

type Config struct {
	Schema string `json:"-" ymal:"-" toml:"-"`
	Server Server
	Clog   clog.Config
}

func (c *Config) GetEnvPtrs() []interface{} {
	return []interface{}{&c.Server, &c.Clog}
}
