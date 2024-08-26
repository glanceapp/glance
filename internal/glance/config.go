package glance

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   Server   `yaml:"server"`
	Theme    Theme    `yaml:"theme"`
	Branding Branding `yaml:"branding"`
	Pages    []Page   `yaml:"pages"`
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

	for p := range config.Pages {
		for c := range config.Pages[p].Columns {
			for w := range config.Pages[p].Columns[c].Widgets {
				if err := config.Pages[p].Columns[c].Widgets[w].Initialize(); err != nil {
					return nil, err
				}
			}
		}
	}

	return config, nil
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

		if config.Pages[i].Width != "" && (config.Pages[i].Width != "wide" && config.Pages[i].Width != "slim") {
			return fmt.Errorf("Page %d: width can only be either wide or slim", i+1)
		}

		if len(config.Pages[i].Columns) == 0 {
			return fmt.Errorf("Page %d has no columns", i+1)
		}

		if config.Pages[i].Width == "slim" {
			if len(config.Pages[i].Columns) > 2 {
				return fmt.Errorf("Page %d is slim and cannot have more than 2 columns", i+1)
			}
		} else {
			if len(config.Pages[i].Columns) > 3 {
				return fmt.Errorf("Page %d has more than 3 columns: %d", i+1, len(config.Pages[i].Columns))
			}
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
