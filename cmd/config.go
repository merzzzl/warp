package main

import (
	"flag"
	"os"
	"os/user"

	"gopkg.in/yaml.v2"

	"github.com/merzzzl/warp/internal/protocol/cloudbric"
	"github.com/merzzzl/warp/internal/protocol/ssh"
	"github.com/merzzzl/warp/internal/service"
)

type Config struct {
	SSH       *ssh.Config       `yaml:"ssh"`
	Cloudbric *cloudbric.Config `yaml:"cloudbric"`
	Tunnel    *service.Config   `yaml:"tunnel"`
	verbose   bool
}

func loadConfig() (*Config, error) {
	var cfg Config

	name, _ := os.LookupEnv("SUDO_USER")

	usr, err := user.Lookup(name)
	if err != nil {
		return nil, err
	}

	flag.BoolVar(&cfg.verbose, "verbose", false, "enable verbose logging (default: disabled)")
	flag.Parse()

	file, err := os.ReadFile(usr.HomeDir + "/" + ".warp.yaml")
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(file, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
