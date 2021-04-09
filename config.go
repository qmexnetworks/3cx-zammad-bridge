package zammadbridge

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Phone3CX struct {
		User            string `yaml:"user"`
		Pass            string `yaml:"pass"`
		Host            string `yaml:"host"`
		Group           string `yaml:"group"`
		ExtensionDigits int    `yaml:"extension_digits"`
		TrunkDigits     int    `yaml:"trunk_digits"`
		QueueExtension  int    `yaml:"queue_extension"`
	} `yaml:"3CX"`
	Zammad struct {
		Endpoint            string `yaml:"endpoint"`
		LogMissedQueueCalls bool   `yaml:"log_missed_queue_calls"`
	} `yaml:"Zammad"`
}

// LoadConfigFromYaml tries the provided files for a valid YAML configuration file.
// It uses the first file it can parse, and only that file.
func LoadConfigFromYaml(filenames ...string) (*Config, error) {
	config := new(Config)

	for _, f := range filenames {
		b, err := os.ReadFile(f)
		if err != nil {
			continue // hopefully other files will work out?
		}

		err = yaml.Unmarshal(b, config)
		if err != nil {
			log.Printf("WARNING: Unable to parse YAML config %s: %q\n", f, err.Error())
			continue // hopefully other files will work out?
		}

		return config, nil
	}

	return nil, fmt.Errorf("unable to find configuration files")
}
