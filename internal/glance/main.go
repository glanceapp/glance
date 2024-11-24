package glance

import (
	"fmt"
	"log"
)

func Main() int {
	options, err := parseCliOptions()

	if err != nil {
		fmt.Println(err)
		return 1
	}

	switch options.intent {
	case cliIntentServe:
		if err := serveApp(options.configPath); err != nil {
			fmt.Println(err)
			return 1
		}
	case cliIntentConfigValidate:
		contents, _, err := parseYAMLIncludes(options.configPath)
		if err != nil {
			fmt.Printf("failed to parse config file: %v\n", err)
			return 1
		}

		if _, err := newConfigFromYAML(contents); err != nil {
			fmt.Printf("config file is invalid: %v\n", err)
			return 1
		}
	case cliIntentConfigPrint:
		contents, _, err := parseYAMLIncludes(options.configPath)
		if err != nil {
			fmt.Printf("failed to parse config file: %v\n", err)
			return 1
		}

		fmt.Println(string(contents))
	case cliIntentDiagnose:
		runDiagnostic()
	}

	return 0
}

func serveApp(configPath string) error {
	exitChannel := make(chan struct{})
	// the onChange method gets called at most once per 500ms due to debouncing so we shouldn't
	// need to use atomic.Bool here unless newConfigFromYAML is very slow for some reason
	hadValidConfigOnStartup := false
	var stopServer func() error

	onChange := func(newContents []byte) {
		if stopServer != nil {
			log.Println("Config file changed, attempting to restart server")
		}

		config, err := newConfigFromYAML(newContents)
		if err != nil {
			log.Printf("Config file is invalid: %v", err)

			if !hadValidConfigOnStartup {
				close(exitChannel)
			}

			return
		} else if !hadValidConfigOnStartup {
			hadValidConfigOnStartup = true
		}

		app := newApplication(config)

		if stopServer != nil {
			if err := stopServer(); err != nil {
				log.Printf("Error while trying to stop server: %v", err)
			}
		}

		go func() {
			var startServer func() error
			startServer, stopServer = app.Server()

			if err := startServer(); err != nil {
				log.Printf("Failed to start server: %v", err)
			}
		}()
	}

	onErr := func(err error) {
		log.Printf("Error watching config files: %v", err)
	}

	configContents, configIncludes, err := parseYAMLIncludes(configPath)
	if err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	stopWatching, err := configFilesWatcher(configPath, configContents, configIncludes, onChange, onErr)
	if err == nil {
		defer stopWatching()
	} else {
		log.Printf("Error starting file watcher, config file changes will require a manual restart. (%v)", err)

		config, err := newConfigFromYAML(configContents)
		if err != nil {
			return fmt.Errorf("could not parse config file: %w", err)
		}

		app := newApplication(config)

		startServer, _ := app.Server()
		if err := startServer(); err != nil {
			return fmt.Errorf("failed to start server: %w", err)
		}
	}

	<-exitChannel
	return nil
}
