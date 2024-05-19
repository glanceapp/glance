package glance

import (
	"fmt"
	"os"
)

func Main() int {
	options, err := ParseCliOptions()

	if err != nil {
		fmt.Println(err)
		return 1
	}

	configFile, err := os.Open(options.ConfigPath)

	if err != nil {
		fmt.Printf("failed opening config file: %v\n", err)
		return 1
	}

	config, err := NewConfigFromYml(configFile)
	configFile.Close()

	if err != nil {
		fmt.Printf("failed parsing config file: %v\n", err)
		return 1
	}

	if options.Intent == CliIntentServe {
		app, err := NewApplication(config)

		if err != nil {
			fmt.Printf("failed creating application: %v\n", err)
			return 1
		}

		if err := app.Serve(); err != nil {
			fmt.Printf("http server error: %v\n", err)
			return 1
		}
	}

	return 0
}
