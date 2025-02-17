package glance

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

type config struct {
	Server struct {
		Host       string    `yaml:"host"`
		Port       uint16    `yaml:"port"`
		AssetsPath string    `yaml:"assets-path"`
		BaseURL    string    `yaml:"base-url"`
		StartedAt  time.Time `yaml:"-"` // used in custom css file
	} `yaml:"server"`

	Document struct {
		Head template.HTML `yaml:"head"`
	} `yaml:"document"`

	Theme struct {
		BackgroundColor          *hslColorField `yaml:"background-color"`
		PrimaryColor             *hslColorField `yaml:"primary-color"`
		PositiveColor            *hslColorField `yaml:"positive-color"`
		NegativeColor            *hslColorField `yaml:"negative-color"`
		Light                    bool           `yaml:"light"`
		ContrastMultiplier       float32        `yaml:"contrast-multiplier"`
		TextSaturationMultiplier float32        `yaml:"text-saturation-multiplier"`
		CustomCSSFile            string         `yaml:"custom-css-file"`
	} `yaml:"theme"`

	Branding struct {
		HideFooter   bool          `yaml:"hide-footer"`
		CustomFooter template.HTML `yaml:"custom-footer"`
		LogoText     string        `yaml:"logo-text"`
		LogoURL      string        `yaml:"logo-url"`
		FaviconURL   string        `yaml:"favicon-url"`
	} `yaml:"branding"`

	Pages []page `yaml:"pages"`
}

type page struct {
	Title                      string `yaml:"name"`
	Slug                       string `yaml:"slug"`
	Width                      string `yaml:"width"`
	ShowMobileHeader           bool   `yaml:"show-mobile-header"`
	ExpandMobilePageNavigation bool   `yaml:"expand-mobile-page-navigation"`
	HideDesktopNavigation      bool   `yaml:"hide-desktop-navigation"`
	CenterVertically           bool   `yaml:"center-vertically"`
	Columns                    []struct {
		Size    string  `yaml:"size"`
		Widgets widgets `yaml:"widgets"`
	} `yaml:"columns"`
	PrimaryColumnIndex int8       `yaml:"-"`
	mu                 sync.Mutex `yaml:"-"`
}

func newConfigFromYAML(contents []byte) (*config, error) {
	contents, err := parseConfigEnvVariables(contents)
	if err != nil {
		return nil, err
	}

	config := &config{}
	config.Server.Port = 8080

	err = yaml.Unmarshal(contents, config)
	if err != nil {
		return nil, err
	}

	if err = isConfigStateValid(config); err != nil {
		return nil, err
	}

	for p := range config.Pages {
		for c := range config.Pages[p].Columns {
			for w := range config.Pages[p].Columns[c].Widgets {
				if err := config.Pages[p].Columns[c].Widgets[w].initialize(); err != nil {
					return nil, formatWidgetInitError(err, config.Pages[p].Columns[c].Widgets[w])
				}
			}
		}
	}

	return config, nil
}

// TODO: change the pattern so that it doesn't match commented out lines
var configEnvVariablePattern = regexp.MustCompile(`(^|.)\$\{([A-Z0-9_]+)\}`)

func parseConfigEnvVariables(contents []byte) ([]byte, error) {
	var err error

	replaced := configEnvVariablePattern.ReplaceAllFunc(contents, func(match []byte) []byte {
		if err != nil {
			return nil
		}

		groups := configEnvVariablePattern.FindSubmatch(match)
		if len(groups) != 3 {
			return match
		}

		prefix, key := string(groups[1]), string(groups[2])
		if prefix == `\` {
			if len(match) >= 2 {
				return match[1:]
			} else {
				return nil
			}
		}

		value, found := os.LookupEnv(key)
		if !found {
			err = fmt.Errorf("environment variable %s not found", key)
			return nil
		}

		return []byte(prefix + value)
	})

	if err != nil {
		return nil, err
	}

	return replaced, nil
}

func formatWidgetInitError(err error, w widget) error {
	return fmt.Errorf("%s widget: %v", w.GetType(), err)
}

var includePattern = regexp.MustCompile(`(?m)^(\s*)!include:\s*(.+)$`)

func parseYAMLIncludes(mainFilePath string) ([]byte, map[string]struct{}, error) {
	mainFileContents, err := os.ReadFile(mainFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading main YAML file: %w", err)
	}

	mainFileAbsPath, err := filepath.Abs(mainFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("getting absolute path of main YAML file: %w", err)
	}
	mainFileDir := filepath.Dir(mainFileAbsPath)

	includes := make(map[string]struct{})
	var includesLastErr error

	mainFileContents = includePattern.ReplaceAllFunc(mainFileContents, func(match []byte) []byte {
		if includesLastErr != nil {
			return nil
		}

		matches := includePattern.FindSubmatch(match)
		if len(matches) != 3 {
			includesLastErr = fmt.Errorf("invalid include match: %v", matches)
			return nil
		}

		indent := string(matches[1])
		includeFilePath := strings.TrimSpace(string(matches[2]))
		if !filepath.IsAbs(includeFilePath) {
			includeFilePath = filepath.Join(mainFileDir, includeFilePath)
		}

		var fileContents []byte
		var err error

		fileContents, err = os.ReadFile(includeFilePath)
		if err != nil {
			includesLastErr = fmt.Errorf("reading included file %s: %w", includeFilePath, err)
			return nil
		}

		includes[includeFilePath] = struct{}{}
		return []byte(prefixStringLines(indent, string(fileContents)))
	})

	if includesLastErr != nil {
		return nil, nil, includesLastErr
	}

	return mainFileContents, includes, nil
}

func configFilesWatcher(
	mainFilePath string,
	lastContents []byte,
	lastIncludes map[string]struct{},
	onChange func(newContents []byte),
	onErr func(error),
) (func() error, error) {
	mainFileAbsPath, err := filepath.Abs(mainFilePath)
	if err != nil {
		return nil, fmt.Errorf("getting absolute path of main file: %w", err)
	}

	// TODO: refactor, flaky
	lastIncludes[mainFileAbsPath] = struct{}{}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating watcher: %w", err)
	}

	updateWatchedFiles := func(previousWatched map[string]struct{}, newWatched map[string]struct{}) {
		for filePath := range previousWatched {
			if _, ok := newWatched[filePath]; !ok {
				watcher.Remove(filePath)
			}
		}

		for filePath := range newWatched {
			if _, ok := previousWatched[filePath]; !ok {
				if err := watcher.Add(filePath); err != nil {
					log.Printf(
						"Could not add file to watcher, changes to this file will not trigger a reload. path: %s, error: %v",
						filePath, err,
					)
				}
			}
		}
	}

	updateWatchedFiles(nil, lastIncludes)

	// needed for lastContents and lastIncludes because they get updated in multiple goroutines
	mu := sync.Mutex{}

	checkForContentChangesBeforeCallback := func() {
		currentContents, currentIncludes, err := parseYAMLIncludes(mainFilePath)
		if err != nil {
			onErr(fmt.Errorf("parsing main file contents for comparison: %w", err))
			return
		}

		// TODO: refactor, flaky
		currentIncludes[mainFileAbsPath] = struct{}{}

		mu.Lock()
		defer mu.Unlock()

		if !maps.Equal(currentIncludes, lastIncludes) {
			updateWatchedFiles(lastIncludes, currentIncludes)
			lastIncludes = currentIncludes
		}

		if !bytes.Equal(lastContents, currentContents) {
			lastContents = currentContents
			onChange(currentContents)
		}
	}

	const debounceDuration = 500 * time.Millisecond
	var debounceTimer *time.Timer
	debouncedCallback := func() {
		if debounceTimer != nil {
			debounceTimer.Stop()
			debounceTimer.Reset(debounceDuration)
		} else {
			debounceTimer = time.AfterFunc(debounceDuration, checkForContentChangesBeforeCallback)
		}
	}

	go func() {
		for {
			select {
			case event, isOpen := <-watcher.Events:
				if !isOpen {
					return
				}
				if event.Has(fsnotify.Write) {
					debouncedCallback()
				} else if event.Has(fsnotify.Rename) {
					// wait for file to be available
					for i := 0; i < 20; i++ {
						_, err := os.Stat(mainFileAbsPath)
						if err == nil {
							break
						}
						time.Sleep(100 * time.Millisecond)
					}
					err := watcher.Add(mainFileAbsPath)
					if err != nil {
						onErr(fmt.Errorf("watching file:", err))
					}
					debouncedCallback()
				} else if event.Has(fsnotify.Remove) {
					func() {
						mu.Lock()
						defer mu.Unlock()
						fileAbsPath, _ := filepath.Abs(event.Name)
						delete(lastIncludes, fileAbsPath)
					}()

					debouncedCallback()
				}
			case err, isOpen := <-watcher.Errors:
				if !isOpen {
					return
				}
				onErr(fmt.Errorf("watcher error: %w", err))
			}
		}
	}()

	onChange(lastContents)

	return func() error {
		if debounceTimer != nil {
			debounceTimer.Stop()
		}

		return watcher.Close()
	}, nil
}

func isConfigStateValid(config *config) error {
	if len(config.Pages) == 0 {
		return fmt.Errorf("no pages configured")
	}

	if config.Server.AssetsPath != "" {
		if _, err := os.Stat(config.Server.AssetsPath); os.IsNotExist(err) {
			return fmt.Errorf("assets directory does not exist: %s", config.Server.AssetsPath)
		}
	}

	for i := range config.Pages {
		if config.Pages[i].Title == "" {
			return fmt.Errorf("page %d has no name", i+1)
		}

		if config.Pages[i].Width != "" && (config.Pages[i].Width != "wide" && config.Pages[i].Width != "slim") {
			return fmt.Errorf("page %d: width can only be either wide or slim", i+1)
		}

		if len(config.Pages[i].Columns) == 0 {
			return fmt.Errorf("page %d has no columns", i+1)
		}

		if config.Pages[i].Width == "slim" {
			if len(config.Pages[i].Columns) > 2 {
				return fmt.Errorf("page %d is slim and cannot have more than 2 columns", i+1)
			}
		} else {
			if len(config.Pages[i].Columns) > 3 {
				return fmt.Errorf("page %d has more than 3 columns", i+1)
			}
		}

		columnSizesCount := make(map[string]int)

		for j := range config.Pages[i].Columns {
			if config.Pages[i].Columns[j].Size != "small" && config.Pages[i].Columns[j].Size != "full" {
				return fmt.Errorf("column %d of page %d: size can only be either small or full", j+1, i+1)
			}

			columnSizesCount[config.Pages[i].Columns[j].Size]++
		}

		full := columnSizesCount["full"]

		if full > 2 || full == 0 {
			return fmt.Errorf("page %d must have either 1 or 2 full width columns", i+1)
		}
	}

	return nil
}
