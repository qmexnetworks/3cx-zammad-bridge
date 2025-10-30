package main

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	zammadbridge "github.com/qmexnetworks/3cx-zammad-bridge"
)

var (
	client *zammadbridge.ZammadBridge
)

var rootCmd = &cobra.Command{
	Use:          "zammadbridge",
	Short:        "3cx-zammad-bridge is a bridge that listens on 3cx to forward information to zammad",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Do Stuff Here
		err := client.Listen()
		if err != nil {
			return err
			//log.Fatalln("Fatal error:", err.Error())
		}

		return nil
	},
}

var (
	verboseMode          bool
	traceMode            bool
	logFormat            string
	customConfigLocation string
)

func setupLogging() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if verboseMode {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	if traceMode {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	}
	if logFormat == "plain" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.DateTime})
	}

	zerolog.FormattedLevels = map[zerolog.Level]string{
		zerolog.TraceLevel: "TRACE",
		zerolog.DebugLevel: "DEBUG",
		zerolog.InfoLevel:  "INFO",
		zerolog.WarnLevel:  "WARN",
		zerolog.ErrorLevel: "ERROR",
		zerolog.FatalLevel: "FATAL",
		zerolog.PanicLevel: "PANIC",
	}
}

func main() {
	rootCmd.PersistentFlags().BoolVarP(&verboseMode, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVarP(&traceMode, "trace", "", false, "trace output, super verbose")
	rootCmd.PersistentFlags().StringVarP(&logFormat, "log-format", "f", "json", "log format: \"json\" or \"plain\"")
	rootCmd.PersistentFlags().StringVarP(&customConfigLocation, "config", "c", "", "custom config file path (default \"/etc/3cx-zammad-bridge/config.yaml\")")
	_ = rootCmd.ParseFlags(os.Args)

	setupLogging()

	var configLocations = []string{
		"config.yaml",
		"/etc/3cx-zammad-bridge/config.yaml",
		"//3cx-zammad-bridge/config.yaml",
	}
	if customConfigLocation != "" {
		configLocations = []string{customConfigLocation}
	}

	c, err := zammadbridge.LoadConfigFromYaml(
		configLocations...,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to load configuration")
	}

	client, err = zammadbridge.NewZammadBridge(c)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to create Zammad bridge")
	}

	err = rootCmd.Execute()
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to execute command")
		os.Exit(1)
	}
}
