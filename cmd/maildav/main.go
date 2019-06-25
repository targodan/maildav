package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/targodan/maildav"

	"github.com/tarent/logrus"
	"github.com/targodan/go-errors"
	"gopkg.in/urfave/cli.v1"
)

func main() {
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config,c",
			Usage: "Path to config file.",
			Value: "config.yml",
		},
	}
	app.Action = func(c *cli.Context) error {
		cfgFile, err := os.OpenFile(c.String("config"), os.O_RDONLY, 0644)
		if err != nil {
			return errors.Wrap("Could not open config file", err)
		}

		cfg, err := maildav.ParseConfig(cfgFile)
		cfgFile.Close()
		if err != nil {
			return errors.Wrap("Error parsing config", err)
		}

		results := make(chan error, len(cfg.Pollers))
		pollerCancels := []func(){}
		for _, pollerCfg := range cfg.Pollers {
			poller, err := maildav.NewPoller(pollerCfg)
			if err != nil {
				return errors.Wrap("Error initializing poller", err)
			}

			uploader := &maildav.Uploader{}

			ctx, cancel := context.WithCancel(context.Background())
			pollerCancels = append(pollerCancels, cancel)

			go func() {
				results <- poller.StartPolling(ctx, uploader)
			}()
		}

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt)

		go func() {
			<-sigChan

			for _, cancel := range pollerCancels {
				cancel()
			}
		}()

		var errs error
		for range pollerCancels {
			errs = errors.NewMultiError(errs, <-results)
		}

		return errs
	}

	err := app.Run(os.Args)
	if err != nil {
		logrus.Fatal(err)
	}
}
