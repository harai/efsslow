package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/harai/efsslow/slow"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func createLogger() *zap.Logger {
	log, err := zap.Config{
		Level:         zap.NewAtomicLevelAt(zap.DebugLevel),
		Development:   true,
		DisableCaller: true,
		Encoding:      "json",
		EncoderConfig: zapcore.EncoderConfig{
			LevelKey:       "level",
			MessageKey:     "message",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}.Build()
	if err != nil {
		panic(err)
	}

	return log
}

func main() {
	app := cli.NewApp()
	app.Name = "efsslow"
	app.Usage = "Detect slow EFS access"

	app.Flags = []cli.Flag{
		&cli.UintFlag{
			Name:    "slow-threshold-ms",
			Aliases: []string{"t"},
			Value:   100,
			Usage:   "Slow threshold",
		},
		&cli.UintFlag{
			Name:    "sample-ratio",
			Aliases: []string{"r"},
			Value:   1000,
			Usage:   "Random sampling ratio",
		},
		&cli.StringFlag{
			Name:    "file-name",
			Aliases: []string{"f"},
			Value:   "",
			Usage:   "Traces which contain this file name are always shown.",
		},
		&cli.UintFlag{
			Name:  "bpf-debug",
			Value: 0,
			Usage: "Enable debug output: bcc.DEBUG_SOURCE: 8, bcc.DEBUG_PREPROCESSOR: 4.",
		},
		&cli.BoolFlag{
			Name:  "debug",
			Value: false,
			Usage: "Show debug output. This argument is different from bpf-debug.",
		},
		&cli.BoolFlag{
			Name:  "quit",
			Value: false,
			Usage: "Quit without tracing. This is mainly for debugging.",
		},
	}

	app.Action = func(c *cli.Context) error {
		log := createLogger()
		defer log.Sync()

		cfg := &slow.Config{
			SlowThresholdMS: c.Uint("slow-threshold-ms"),
			SampleRatio:     c.Uint("sample-ratio"),
			BpfDebug:        c.Uint("bpf-debug"),
			FileName:        c.String("file-name"),
			Debug:           c.Bool("debug"),
			Quit:            c.Bool("quit"),
			Log:             log,
		}

		ctx, cancel := context.WithCancel(context.Background())

		sig := make(chan os.Signal)
		signal.Notify(sig, os.Interrupt, os.Kill)
		go func() {
			<-sig
			cancel()
		}()

		slow.Run(ctx, cfg)

		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal("failed to run app", zap.Error(err))
	}
}
