package maildav

import (
	"io"
	"time"

    errors "github.com/targodan/go-errors"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Sources      []*SourceConfig      `yaml:"sources"`
	Destinations []*DestinationConfig `yaml:"destinations"`
	Pollers      []*PollerConfig      `yaml:"pollers"`
}

type SourceConfig struct {
	Name     string `yaml:"name"`
	Server   string `yaml:"server"`
	Port     int    `yaml:"port"`
	TLS      bool   `yaml:"tls"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type DestinationConfig struct {
	Name     string `yaml:"name"`
	BaseURL  string `yaml:"baseURL"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// TODO: Maybe support multiple destinations per poller
//      otherwise uploading to multiple destinations is impossible
//      as message get flagged as read immediately upon download.

type PollerConfig struct {
	SourceName           string `yaml:"source"`
	SourceConfig         *SourceConfig
	SourceDirectories    []string `yaml:"sourceDirectories"`
	DestinationName      string   `yaml:"destination"`
	DestinationConfig    *DestinationConfig
	DestinationDirectory string        `yaml:"destinationDirectory"`
	Timeout              time.Duration `yaml:"timeout"`
}

func ParseConfig(rdr io.Reader) (*Config, error) {
	decoder := yaml.NewDecoder(rdr)
    decoder.SetStrict(true)

	cfg := &Config{}
	err := decoder.Decode(cfg)
	if err != nil {
		return nil, err
	}
	err = cfg.mapSourcesAndDestinations()
	if err != nil {
		return nil, err
	}
	return cfg, cfg.verifyTimeouts()
}

func (cfg *Config) findSource(name string) *SourceConfig {
	for _, c := range cfg.Sources {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func (cfg *Config) findDestination(name string) *DestinationConfig {
	for _, c := range cfg.Destinations {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func (cfg *Config) mapSourcesAndDestinations() error {
	for _, poller := range cfg.Pollers {
		poller.SourceConfig = cfg.findSource(poller.SourceName)
		if poller.SourceConfig == nil {
			return errors.New("unknown source \"" + poller.SourceName + "\"")
		}

		poller.DestinationConfig = cfg.findDestination(poller.DestinationName)
		if poller.DestinationConfig == nil {
			return errors.New("unknown destination \"" + poller.DestinationName + "\"")
		}
	}
	return nil
}

func (cfg *Config) verifyTimeouts() error {
    var err error

	for _, poller := range cfg.Pollers {
		if poller.Timeout <= 0 {
            err = errors.NewMultiError(err, errors.Newf("invalid timeout %v for poller %s -> %s", poller.Timeout, poller.SourceName, poller.DestinationName))
		}
	}

	return err
}
