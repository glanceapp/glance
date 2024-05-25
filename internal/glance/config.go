package glance

import (
	"errors"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server Server `yaml:"server"`
	Theme  Theme  `yaml:"theme"`
	Pages  []Page `yaml:"pages"`
}

func NewConfigFromYml(contents io.Reader) (*Config, error) {
	config := NewConfig()

	contentBytes, err := io.ReadAll(contents)

	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(contentBytes, config)

	if err != nil {
		return nil, err
	}

	if err = configIsValid(config); err != nil {
		return nil, err
	}

	return config, nil
}

// NewConfigFromFile reads a yaml file and returns a Config struct
func NewConfigFromFile(path string) (*Config, error) {
	configFile, err := os.Open(path)

	if err != nil {
		return nil, errors.New("failed opening config file: " + err.Error())
	}

	defer configFile.Close()

	return NewConfigFromYml(configFile)
}

func NewConfig() *Config {
	config := &Config{}

	config.Server.Host = ""
	config.Server.Port = 8080

	return config
}

func configIsValid(config *Config) error {
	for i := range config.Pages {
		if config.Pages[i].Title == "" {
			return fmt.Errorf("Page %d has no title", i+1)
		}

		if len(config.Pages[i].Columns) == 0 {
			return fmt.Errorf("Page %d has no columns", i+1)
		}

		if len(config.Pages[i].Columns) > 3 {
			return fmt.Errorf("Page %d has more than 3 columns: %d", i+1, len(config.Pages[i].Columns))
		}

		columnSizesCount := make(map[string]int)

		for j := range config.Pages[i].Columns {
			if config.Pages[i].Columns[j].Size != "small" && config.Pages[i].Columns[j].Size != "full" {
				return fmt.Errorf("Column %d of page %d: size can only be either small or full", j+1, i+1)
			}

			columnSizesCount[config.Pages[i].Columns[j].Size]++
		}

		full := columnSizesCount["full"]

		if full > 2 || full == 0 {
			return fmt.Errorf("Page %d must have either 1 or 2 full width columns", i+1)
		}
	}

	return nil
}

func watchConfigFile(configPath string, f func(*Config) error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Printf("failed creating watcher: %v\n", err)
		return
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					config, err := NewConfigFromFile(configPath)
					if err != nil {
						fmt.Printf("failed loading config file: %v\n", err)
						return
					}
					err = f(config)
					if err != nil {
						fmt.Printf("failed applying new config: %v\n", err)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				_ = watcher.Close()
				fmt.Printf("watcher error: %v\n", err)
			}
		}
	}()

	err = watcher.Add(configPath)
	if err != nil {
		fmt.Printf("failed adding watcher: %v\n", err)
	}
}
