package main

import (
	"net/url"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	urfaveCli "github.com/urfave/cli/v2"
)

// Version (may be overrided during build)
var version = "develop"

func cli(callback func(appConfig)) {
	app := &urfaveCli.App{
		Name:      "go-transproxy",
		Usage:     "because proxies are a pain in the ass",
		HideHelp:  true,
		ArgsUsage: "<pac-file-uri>",
		Version:   version,
		Flags: []urfaveCli.Flag{
			&urfaveCli.BoolFlag{
				Name:    "debug",
				Aliases: []string{"d"},
				Usage:   "Debug mode",
				Value:   false,
			},
			&urfaveCli.BoolFlag{
				Name:    "trace",
				Aliases: []string{"dd", "ddd"},
				Usage:   "Verbose mode",
				Value:   false,
			},
			&urfaveCli.StringFlag{
				Name:    "timeout",
				Aliases: []string{"t"},
				Usage:   "Connection timeout on TCP connections",
				Value:   "30s",
			},
			&urfaveCli.StringFlag{
				Name:    "pac-file-timeout",
				Aliases: []string{"pft"},
				Usage:   "Connection timeout on PAC file requests",
				Value:   "2s",
			},
			&urfaveCli.StringFlag{
				Name:    "pac-file-ttl",
				Aliases: []string{"pttl"},
				Usage:   "TTL on PAC file",
				Value:   "60s",
			},
			&urfaveCli.IntFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Usage:   "Listening port",
				Value:   3128,
			},
			&urfaveCli.BoolFlag{
				Name:    "tunnel",
				Aliases: []string{"c"},
				Usage:   "Tunnel HTTP request with HTTP CONNECT",
				Value:   true,
			},
		},
		Action: func(c *urfaveCli.Context) error {
			if c.NArg() != 1 {
				urfaveCli.ShowAppHelpAndExit(c, 1)
			}
			logLevel := log.InfoLevel
			if c.Bool("trace") {
				logLevel = log.TraceLevel
			} else if c.Bool("debug") {
				logLevel = log.DebugLevel
			}
			timeout, err := time.ParseDuration(c.String("timeout"))
			if err != nil {
				log.Fatal(err)
			}
			pacFileTimeout, err := time.ParseDuration(c.String("pac-file-timeout"))
			if err != nil {
				log.Fatal(err)
			}
			pacFileTTL, err := time.ParseDuration(c.String("pac-file-ttl"))
			if err != nil {
				log.Fatal(err)
			}
			proxyPacURI, err := url.Parse(c.Args().First())
			if err != nil {
				log.Fatal(err)
			}

			config := appConfig{
				logLevel:       logLevel,
				timeout:        timeout,
				pacFileTimeout: pacFileTimeout,
				pacFileTTL:     pacFileTTL,
				port:           c.Int("port"),
				proxyPacURI:    proxyPacURI,
				tunnel:         c.Bool("tunnel"),
			}

			callback(config)
			return nil
		},
	}

	app.Run(os.Args)
}
