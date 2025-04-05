package glance

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

var buildVersion = "dev"

func Main() int {
	options, err := parseCliOptions()
	if err != nil {
		fmt.Println(err)
		return 1
	}

	switch options.intent {
	case cliIntentServe:
		// remove in v0.10.0
		if serveUpdateNoticeIfConfigLocationNotMigrated(options.configPath) {
			return 1
		}

		if err := serveApp(options.configPath); err != nil {
			fmt.Println(err)
			return 1
		}
	case cliIntentConfigValidate:
		contents, _, err := parseYAMLIncludes(options.configPath)
		if err != nil {
			fmt.Printf("Could not parse config file: %v\n", err)
			return 1
		}

		if _, err := newConfigFromYAML(contents); err != nil {
			fmt.Printf("Config file is invalid: %v\n", err)
			return 1
		}
	case cliIntentConfigPrint:
		contents, _, err := parseYAMLIncludes(options.configPath)
		if err != nil {
			fmt.Printf("Could not parse config file: %v\n", err)
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
			log.Println("Config file changed, reloading...")
		}

		config, err := newConfigFromYAML(newContents)
		if err != nil {
			log.Printf("Config has errors: %v", err)

			if !hadValidConfigOnStartup {
				close(exitChannel)
			}

			return
		} else if !hadValidConfigOnStartup {
			hadValidConfigOnStartup = true
		}

		app, err := newApplication(config)
		if err != nil {
			log.Printf("Failed to create application: %v", err)
			return
		}

		if stopServer != nil {
			if err := stopServer(); err != nil {
				log.Printf("Error while trying to stop server: %v", err)
			}
		}

		go func() {
			var startServer func() error
			startServer, stopServer = app.server()

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
		return fmt.Errorf("parsing config: %w", err)
	}

	stopWatching, err := configFilesWatcher(configPath, configContents, configIncludes, onChange, onErr)
	if err == nil {
		defer stopWatching()
	} else {
		log.Printf("Error starting file watcher, config file changes will require a manual restart. (%v)", err)

		config, err := newConfigFromYAML(configContents)
		if err != nil {
			return fmt.Errorf("validating config file: %w", err)
		}

		app, err := newApplication(config)
		if err != nil {
			return fmt.Errorf("creating application: %w", err)
		}

		startServer, _ := app.server()
		if err := startServer(); err != nil {
			return fmt.Errorf("starting server: %w", err)
		}
	}

	<-exitChannel
	return nil
}

func serveUpdateNoticeIfConfigLocationNotMigrated(configPath string) bool {
	if !isRunningInsideDockerContainer() {
		return false
	}

	if _, err := os.Stat(configPath); err == nil {
		return false
	}

	// glance.yml wasn't mounted to begin with or was incorrectly mounted as a directory
	if stat, err := os.Stat("glance.yml"); err != nil || stat.IsDir() {
		return false
	}

	templateFile, _ := templateFS.Open("v0.7-update-notice-page.html")
	bodyContents, _ := io.ReadAll(templateFile)

	// TODO: update - add link
	fmt.Println("!!! WARNING !!!")
	fmt.Println("The default location of glance.yml in the Docker image has changed starting from v0.7.0, please see <link> for more information.")

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(bodyContents))
	})

	server := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	server.ListenAndServe()

	return true
}
