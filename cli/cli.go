package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/harai/efsslow/slow"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

func createLogger() *zap.Logger {
	log, err := zap.NewDevelopment(zap.WithCaller(false))
	if err != nil {
		panic("failed to initialize logger")
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
			BpfDebug:        c.Uint("bpf-debug"),
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
