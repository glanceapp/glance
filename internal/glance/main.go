package glance

import (
	"fmt"
)

func Main() int {
	options, err := ParseCliOptions()

	if err != nil {
		fmt.Println(err)
		return 1
	}

	config, err := NewConfigFromFile(options.ConfigPath)
	if err != nil {
		fmt.Printf("failed loading config file: %v\n", err)
		return 1
	}

	if options.Intent == CliIntentServe {
		app, err := NewApplication(config)

		if err != nil {
			fmt.Printf("failed creating application: %v\n", err)
			return 1
		}

		watchConfigFile(options.ConfigPath, func(config *Config) error {
			app.Reload(config)
			return nil
		})

		if app.Serve() != nil {
			fmt.Printf("http server error: %v\n", err)
			return 1
		}
	}

	return 0
}
