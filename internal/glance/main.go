package glance

import (
	"fmt"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
)

var (
	currentApp *Application
	done       chan bool
)

func Main() int {
	options, err := ParseCliOptions()

	if err != nil {
		fmt.Println(err)
		return 1
	}

	if options.Intent == CliIntentServe {
		err := startWatcherAndApp(options.ConfigPath, false)
		if err != nil {
			fmt.Println(err)
			return 1
		}
	}

	return 0
}

func startWatcherAndApp(configPath string, sendReload bool) error {
	done = make(chan bool)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %v", err)
	}
	defer watcher.Close()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					fmt.Println("config file modified, restarting application...")
					if currentApp != nil {
						if err := currentApp.Stop(); err != nil {
							fmt.Printf("failed to shutdown application: %v\n", err)
						}
						time.Sleep(1 * time.Second) // give it enough time to shutdown
					}
					startWatcherAndApp(configPath, true)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Printf("error watching config file: %v\n", err)
			}
		}
	}()

	err = watcher.Add(configPath)
	if err != nil {
		return fmt.Errorf("failed to watch config file: %v", err)
	}

	restartApplication(configPath, sendReload)
	<-done

	return nil
}

func restartApplication(configPath string, sendReload bool) {
	configFile, err := os.Open(configPath)
	if err != nil {
		fmt.Printf("failed opening config file: %v\n", err)
		return
	}

	config, err := NewConfigFromYml(configFile)
	configFile.Close()
	if err != nil {
		fmt.Printf("failed parsing config file: %v\n", err)
		return
	}

	app, err := NewApplication(config)
	if err != nil {
		fmt.Printf("failed creating application: %v\n", err)
		return
	}

	currentApp = app

	if sendReload {
		wsBroadcast <- []byte("reload")
	}

	if err := app.Serve(); err != nil {
		fmt.Printf("http server error: %v\n", err)
	}

}
