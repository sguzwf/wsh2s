package config

import (
	"reflect"

	"golang.org/x/net/http2"

	"github.com/caarlos0/env"
	"github.com/empirefox/gotool/crypt"
	"github.com/mcuadros/go-defaults"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"gopkg.in/go-playground/validator.v9"
)

type Env struct {
	XpsMode     string `env:"XPS_MODE"`
	XpsPassword string `env:"XPS_PASSWORD"`
}

// LoadFromXpsWithEnv load with CONFIG_MODE
func LoadFromXpsWithEnv() (*Config, error) {
	config := new(Config)

	err := env.Parse(&config.Env)
	if err != nil {
		return nil, err
	}

	opts := &crypt.ConfigOptions{
		Mode:     config.Env.XpsMode,
		Password: config.Env.XpsPassword,
	}
	err = crypt.LoadSingleConfigWithOptions(config, opts)
	if err != nil {
		return nil, err
	}

	// overwrite with env
	err = env.Parse(config)
	if err != nil {
		return nil, err
	}

	validate := validator.New()
	validateRequireField := func(fl validator.FieldLevel) bool {
		field := fl.Field()
		if validate.Var(field.Interface(), "required") != nil {
			return true
		}

		rqField, rqKind, ok := fl.GetStructFieldOK()
		if !ok {
			return false
		}

		rq := rqField.Interface()
		if rqKind == reflect.Slice && rqField.Len() == 0 {
			return false
		}

		if validate.Var(rq, "required") != nil {
			return false
		}
		return true
	}
	validate.RegisterValidation("rq", validateRequireField)
	validate.RegisterAlias("zap_level", "len=0|eq=debug|eq=info|eq=warn|eq=error|eq=dpanic|eq=panic|eq=fatal")
	validate.RegisterAlias("sign_alg", "eq=HS256|eq=HS384|eq=HS512")

	defaults.SetDefaults(config)
	//	color.Red("%d\n", config.Captcha.ExpiresSecond)
	if err = validate.Struct(config); err != nil {
		return nil, err
	}

	var zapConfig zap.Config
	if config.Dev {
		zapConfig = zap.NewDevelopmentConfig()
	} else {
		zapConfig = zap.NewProductionConfig()
	}

	if config.ZapLevel != "" {
		level := new(zapcore.Level)
		level.Set(config.ZapLevel)
		zapConfig.Level.SetLevel(*level)
	}
	config.Logger, err = zapConfig.Build()
	if err != nil {
		return nil, err
	}

	http2.VerboseLogs = config.H2Logs
	return config, nil
}
