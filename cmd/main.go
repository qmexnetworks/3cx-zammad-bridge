package main

import (
	"os"

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

func main() {
	var verboseMode bool
	var traceMode bool
	rootCmd.PersistentFlags().BoolVarP(&verboseMode, "verbose", "v", false, "verbose output -- not for production")
	rootCmd.PersistentFlags().BoolVarP(&traceMode, "trace", "", false, "trace output -- not for production")
	_ = rootCmd.ParseFlags(os.Args)

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if verboseMode {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	if traceMode {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	}

	c, err := zammadbridge.LoadConfigFromYaml(
		"config.yaml",
		"/etc/3cx-zammad-bridge/config.yaml",
		"//3cx-zammad-bridge/config.yaml",
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
