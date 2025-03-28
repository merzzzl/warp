package main

import (
	"errors"
	"flag"
	"os"
	"os/user"
	"reflect"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/merzzzl/warp/internal/protocol/socks5"
	"github.com/merzzzl/warp/internal/protocol/ssh"
	"github.com/merzzzl/warp/internal/protocol/wg"
	"github.com/merzzzl/warp/internal/service"
)

var errInvalidConfig = errors.New("invalid config of protocols")

type ConfigProtocol struct {
	SSH       *ssh.Config    `yaml:"ssh"`
	SOCKS5    *socks5.Config `yaml:"socks5"`
	WireGuard *wg.Config     `yaml:"wireguard"`
}

type Config struct {
	Tunnel    *service.Config  `yaml:"tunnel"`
	Protocols []ConfigProtocol `yaml:"protocols"`
	verbose   bool
	debug     bool
	fun       bool
}

func loadConfig() (*Config, error) {
	var cfg Config

	name, _ := os.LookupEnv("SUDO_USER")

	usr, err := user.Lookup(name)
	if err != nil {
		return nil, err
	}

	flag.BoolVar(&cfg.verbose, "verbose", false, "enable verbose logging (default: disabled)")
	flag.BoolVar(&cfg.debug, "debug", false, "enable debug logging (default: disabled)")
	flag.BoolVar(&cfg.fun, "fun", false, "magic!")
	flag.Parse()

	file, err := os.ReadFile(usr.HomeDir + "/" + ".warp.yaml")
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(file, &cfg); err != nil {
		return nil, err
	}

	if !cfg.validate() {
		return nil, errInvalidConfig
	}

	for _, pConfig := range cfg.Protocols {
		if pConfig.SSH != nil {
			if strings.Contains(pConfig.SSH.User, "radik") {
				cfg.fun = !cfg.fun

				break
			}
		}
	}

	return &cfg, nil
}

func (c *Config) validate() bool {
	for _, p := range c.Protocols {
		if !p.validate() {
			return false
		}
	}

	return true
}

func (c *ConfigProtocol) validate() bool {
	v := reflect.ValueOf(c)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	var find bool

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if field.IsValid() && !field.IsNil() {
			if find {
				return false
			}

			find = true
		}
	}

	return find
}
