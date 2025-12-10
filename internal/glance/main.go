package glance

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"golang.org/x/crypto/bcrypt"
)

var buildVersion = "dev"

func Main() int {
	options, err := parseCliOptions()
	if err != nil {
		fmt.Println(err)
		return 1
	}

	switch options.intent {
	case cliIntentVersionPrint:
		fmt.Println(buildVersion)
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
	case cliIntentSensorsPrint:
		return cliSensorsPrint()
	case cliIntentMountpointInfo:
		return cliMountpointInfo(options.args[1])
	case cliIntentDiagnose:
		runDiagnostic()
	case cliIntentSecretMake:
		key, err := makeAuthSecretKey(AUTH_SECRET_KEY_LENGTH)
		if err != nil {
			fmt.Printf("Failed to make secret key: %v\n", err)
			return 1
		}

		fmt.Println(key)
	case cliIntentPasswordHash:
		password := options.args[1]

		if password == "" {
			fmt.Println("Password cannot be empty")
			return 1
		}

		if len(password) < 6 {
			fmt.Println("Password must be at least 6 characters long")
			return 1
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			fmt.Printf("Failed to hash password: %v\n", err)
			return 1
		}

		fmt.Println(string(hashedPassword))
	}

	return 0
}

func serveApp(configPath string) error {
	// TODO: refactor if this gets any more complex, the current implementation is
	// difficult to reason about due to all of the callbacks and simultaneous operations,
	// use a single goroutine and a channel to initiate synchronous changes to the server
	exitChannel := make(chan struct{})
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
		}

		app, err := newApplication(config)
		if err != nil {
			log.Printf("Failed to create application: %v", err)

			if !hadValidConfigOnStartup {
				close(exitChannel)
			}

			return
		}

		if !hadValidConfigOnStartup {
			hadValidConfigOnStartup = true
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

	fmt.Println("!!! WARNING !!!")
	fmt.Println("The default location of glance.yml in the Docker image has changed starting from v0.7.0.")
	fmt.Println("Please see https://github.com/glanceapp/glance/blob/main/docs/v0.7.0-upgrade.md for more information.")

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
