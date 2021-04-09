package main

import (
	"log"
	"os"

	zammadbridge "github.com/qmexnetworks/3cx-zammad-bridge"
	"github.com/spf13/cobra"
)

var (
	client *zammadbridge.ZammadBridge
)

var rootCmd = &cobra.Command{
	Use:   "zammadbridge",
	Short: "3cx-zammad-bridge is a bridge that listens on 3cx to forward information to zammad",
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
	/*

	- `/etc/3cx-zammad-bridge/config.yaml`
	- `/opt/3cx-zammad-bridge/config.yaml`
	- `config.yaml`  (within the working directory of this 3cx bridge process)

	*/
	c, err := zammadbridge.LoadConfigFromYaml(
		"config.yaml",
		"/etc/3cx-zammad-bridge/config.yaml",
		"//3cx-zammad-bridge/config.yaml",
	)
	if err != nil {
		log.Fatalln(err.Error())
	}

	client, err = zammadbridge.NewZammadBridge(c)
	if err != nil {
		log.Fatalln(err.Error())
	}

	err = rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
