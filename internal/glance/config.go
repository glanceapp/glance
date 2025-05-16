package glance

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"iter"
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

const CONFIG_INCLUDE_RECURSION_DEPTH_LIMIT = 20

const (
	configVarTypeEnv         = "env"
	configVarTypeSecret      = "secret"
	configVarTypeFileFromEnv = "readFileFromEnv"
)

type config struct {
	Server struct {
		Host       string `yaml:"host"`
		Port       uint16 `yaml:"port"`
		Proxied    bool   `yaml:"proxied"`
		AssetsPath string `yaml:"assets-path"`
		BaseURL    string `yaml:"base-url"`
	} `yaml:"server"`

	Auth struct {
		SecretKey string           `yaml:"secret-key"`
		Users     map[string]*user `yaml:"users"`
	} `yaml:"auth"`

	Document struct {
		Head template.HTML `yaml:"head"`
	} `yaml:"document"`

	Theme struct {
		themeProperties `yaml:",inline"`
		CustomCSSFile   string `yaml:"custom-css-file"`

		DisablePicker bool                                     `yaml:"disable-picker"`
		Presets       orderedYAMLMap[string, *themeProperties] `yaml:"presets"`
	} `yaml:"theme"`

	Branding struct {
		HideFooter         bool          `yaml:"hide-footer"`
		CustomFooter       template.HTML `yaml:"custom-footer"`
		LogoText           string        `yaml:"logo-text"`
		LogoURL            string        `yaml:"logo-url"`
		FaviconURL         string        `yaml:"favicon-url"`
		FaviconType        string        `yaml:"-"`
		AppName            string        `yaml:"app-name"`
		AppIconURL         string        `yaml:"app-icon-url"`
		AppBackgroundColor string        `yaml:"app-background-color"`
	} `yaml:"branding"`

	Pages []page `yaml:"pages"`
}

type user struct {
	Password           string `yaml:"password"`
	PasswordHashString string `yaml:"password-hash"`
	PasswordHash       []byte `yaml:"-"`
}

type page struct {
	Title                  string  `yaml:"name"`
	Slug                   string  `yaml:"slug"`
	Width                  string  `yaml:"width"`
	DesktopNavigationWidth string  `yaml:"desktop-navigation-width"`
	ShowMobileHeader       bool    `yaml:"show-mobile-header"`
	HideDesktopNavigation  bool    `yaml:"hide-desktop-navigation"`
	CenterVertically       bool    `yaml:"center-vertically"`
	HeadWidgets            widgets `yaml:"head-widgets"`
	Columns                []struct {
		Size    string  `yaml:"size"`
		Widgets widgets `yaml:"widgets"`
	} `yaml:"columns"`
	PrimaryColumnIndex int8       `yaml:"-"`
	mu                 sync.Mutex `yaml:"-"`
}

func newConfigFromYAML(contents []byte) (*config, error) {
	contents, err := parseConfigVariables(contents)
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
		for w := range config.Pages[p].HeadWidgets {
			if err := config.Pages[p].HeadWidgets[w].initialize(); err != nil {
				return nil, formatWidgetInitError(err, config.Pages[p].HeadWidgets[w])
			}
		}

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

var envVariableNamePattern = regexp.MustCompile(`^[A-Z0-9_]+$`)
var configVariablePattern = regexp.MustCompile(`(^|.)\$\{(?:([a-zA-Z]+):)?([a-zA-Z0-9_-]+)\}`)

// Parses variables defined in the config such as:
// ${API_KEY} 				            - gets replaced with the value of the API_KEY environment variable
// \${API_KEY} 					        - escaped, gets used as is without the \ in the config
// ${secret:api_key} 			        - value gets loaded from /run/secrets/api_key
// ${readFileFromEnv:PATH_TO_SECRET}    - value gets loaded from the file path specified in the environment variable PATH_TO_SECRET
//
// TODO: don't match against commented out sections, not sure exactly how since
// variables can be placed anywhere and used to modify the YAML structure itself
func parseConfigVariables(contents []byte) ([]byte, error) {
	var err error

	replaced := configVariablePattern.ReplaceAllFunc(contents, func(match []byte) []byte {
		if err != nil {
			return nil
		}

		groups := configVariablePattern.FindSubmatch(match)
		if len(groups) != 4 {
			// we can't handle this match, this shouldn't happen unless the number of groups
			// in the regex has been changed without updating the below code
			return match
		}

		prefix := string(groups[1])
		if prefix == `\` {
			if len(match) >= 2 {
				return match[1:]
			} else {
				return nil
			}
		}

		typeAsString, variableName := string(groups[2]), string(groups[3])
		variableType := ternary(typeAsString == "", configVarTypeEnv, typeAsString)

		parsedValue, returnOriginal, localErr := parseConfigVariableOfType(variableType, variableName)
		if localErr != nil {
			err = fmt.Errorf("parsing variable: %v", localErr)
			return nil
		}

		if returnOriginal {
			return match
		}

		return []byte(prefix + parsedValue)
	})

	if err != nil {
		return nil, err
	}

	return replaced, nil
}

// When the bool return value is true, it indicates that the caller should use the original value
func parseConfigVariableOfType(variableType, variableName string) (string, bool, error) {
	switch variableType {
	case configVarTypeEnv:
		if !envVariableNamePattern.MatchString(variableName) {
			return "", true, nil
		}

		v, found := os.LookupEnv(variableName)
		if !found {
			return "", false, fmt.Errorf("environment variable %s not found", variableName)
		}

		return v, false, nil
	case configVarTypeSecret:
		secretPath := filepath.Join("/run/secrets", variableName)
		secret, err := os.ReadFile(secretPath)
		if err != nil {
			return "", false, fmt.Errorf("reading secret file: %v", err)
		}

		return strings.TrimSpace(string(secret)), false, nil
	case configVarTypeFileFromEnv:
		if !envVariableNamePattern.MatchString(variableName) {
			return "", true, nil
		}

		filePath, found := os.LookupEnv(variableName)
		if !found {
			return "", false, fmt.Errorf("readFileFromEnv: environment variable %s not found", variableName)
		}

		if !filepath.IsAbs(filePath) {
			return "", false, fmt.Errorf("readFileFromEnv: file path %s is not absolute", filePath)
		}

		fileContents, err := os.ReadFile(filePath)
		if err != nil {
			return "", false, fmt.Errorf("readFileFromEnv: reading file from %s: %v", variableName, err)
		}

		return strings.TrimSpace(string(fileContents)), false, nil
	default:
		return "", true, nil
	}
}

func formatWidgetInitError(err error, w widget) error {
	return fmt.Errorf("%s widget: %v", w.GetType(), err)
}

var configIncludePattern = regexp.MustCompile(`(?m)^([ \t]*)(?:-[ \t]*)?(?:!|\$)include:[ \t]*(.+)$`)

func parseYAMLIncludes(mainFilePath string) ([]byte, map[string]struct{}, error) {
	return recursiveParseYAMLIncludes(mainFilePath, nil, 0)
}

func recursiveParseYAMLIncludes(mainFilePath string, includes map[string]struct{}, depth int) ([]byte, map[string]struct{}, error) {
	if depth > CONFIG_INCLUDE_RECURSION_DEPTH_LIMIT {
		return nil, nil, fmt.Errorf("recursion depth limit of %d reached", CONFIG_INCLUDE_RECURSION_DEPTH_LIMIT)
	}

	mainFileContents, err := os.ReadFile(mainFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", mainFilePath, err)
	}

	mainFileAbsPath, err := filepath.Abs(mainFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("getting absolute path of %s: %w", mainFilePath, err)
	}
	mainFileDir := filepath.Dir(mainFileAbsPath)

	if includes == nil {
		includes = make(map[string]struct{})
	}
	var includesLastErr error

	mainFileContents = configIncludePattern.ReplaceAllFunc(mainFileContents, func(match []byte) []byte {
		if includesLastErr != nil {
			return nil
		}

		matches := configIncludePattern.FindSubmatch(match)
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

		includes[includeFilePath] = struct{}{}

		fileContents, includes, err = recursiveParseYAMLIncludes(includeFilePath, includes, depth+1)
		if err != nil {
			includesLastErr = err
			return nil
		}

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

	parseAndCompareBeforeCallback := func() {
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
	debouncedParseAndCompareBeforeCallback := func() {
		if debounceTimer != nil {
			debounceTimer.Stop()
			debounceTimer.Reset(debounceDuration)
		} else {
			debounceTimer = time.AfterFunc(debounceDuration, parseAndCompareBeforeCallback)
		}
	}

	deleteLastInclude := func(filePath string) {
		mu.Lock()
		defer mu.Unlock()
		fileAbsPath, _ := filepath.Abs(filePath)
		delete(lastIncludes, fileAbsPath)
	}

	go func() {
		for {
			select {
			case event, isOpen := <-watcher.Events:
				if !isOpen {
					return
				}
				if event.Has(fsnotify.Write) {
					debouncedParseAndCompareBeforeCallback()
				} else if event.Has(fsnotify.Rename) {
					// on linux the file will no longer be watched after a rename, on windows
					// it will continue to be watched with the new name but we have no access to
					// the new name in this event in order to stop watching it manually and match the
					// behavior in linux, may lead to weird unintended behaviors on windows as we're
					// only handling renames from linux's perspective
					// see https://github.com/fsnotify/fsnotify/issues/255

					// remove the old file from our manually tracked includes, calling
					// debouncedParseAndCompareBeforeCallback will re-add it if it's still
					// required after it triggers
					deleteLastInclude(event.Name)

					// wait for file to maybe get created again
					// see https://github.com/glanceapp/glance/pull/358
					for range 10 {
						if _, err := os.Stat(event.Name); err == nil {
							break
						}
						time.Sleep(200 * time.Millisecond)
					}

					debouncedParseAndCompareBeforeCallback()
				} else if event.Has(fsnotify.Remove) {
					deleteLastInclude(event.Name)
					debouncedParseAndCompareBeforeCallback()
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

// TODO: Refactor, we currently validate in two different places, this being
// one of them, which doesn't modify the data and only checks for logical errors
// and then again when creating the application which does modify the data and do
// further validation. Would be better if validation was done in a single place.
func isConfigStateValid(config *config) error {
	if len(config.Pages) == 0 {
		return fmt.Errorf("no pages configured")
	}

	if len(config.Auth.Users) > 0 && config.Auth.SecretKey == "" {
		return fmt.Errorf("secret-key must be set when users are configured")
	}

	for username := range config.Auth.Users {
		if username == "" {
			return fmt.Errorf("user has no name")
		}

		if len(username) < 3 {
			return errors.New("usernames must be at least 3 characters")
		}

		user := config.Auth.Users[username]

		if user.Password == "" {
			if user.PasswordHashString == "" {
				return fmt.Errorf("user %s must have a password or a password-hash set", username)
			}
		} else if len(user.Password) < 6 {
			return fmt.Errorf("the password for %s must be at least 6 characters", username)
		}
	}

	if config.Server.AssetsPath != "" {
		if _, err := os.Stat(config.Server.AssetsPath); os.IsNotExist(err) {
			return fmt.Errorf("assets directory does not exist: %s", config.Server.AssetsPath)
		}
	}

	for i := range config.Pages {
		page := &config.Pages[i]

		if page.Title == "" {
			return fmt.Errorf("page %d has no name", i+1)
		}

		if page.Width != "" && (page.Width != "wide" && page.Width != "slim" && page.Width != "default") {
			return fmt.Errorf("page %d: width can only be either wide or slim", i+1)
		}

		if page.DesktopNavigationWidth != "" {
			if page.DesktopNavigationWidth != "wide" && page.DesktopNavigationWidth != "slim" && page.DesktopNavigationWidth != "default" {
				return fmt.Errorf("page %d: desktop-navigation-width can only be either wide or slim", i+1)
			}
		}

		if len(page.Columns) == 0 {
			return fmt.Errorf("page %d has no columns", i+1)
		}

		if page.Width == "slim" {
			if len(page.Columns) > 2 {
				return fmt.Errorf("page %d is slim and cannot have more than 2 columns", i+1)
			}
		} else {
			if len(page.Columns) > 3 {
				return fmt.Errorf("page %d has more than 3 columns", i+1)
			}
		}

		columnSizesCount := make(map[string]int)

		for j := range page.Columns {
			column := &page.Columns[j]

			if column.Size != "small" && column.Size != "full" {
				return fmt.Errorf("column %d of page %d: size can only be either small or full", j+1, i+1)
			}

			columnSizesCount[page.Columns[j].Size]++
		}

		full := columnSizesCount["full"]

		if full > 2 || full == 0 {
			return fmt.Errorf("page %d must have either 1 or 2 full width columns", i+1)
		}
	}

	return nil
}

// Read-only way to store ordered maps from a YAML structure
type orderedYAMLMap[K comparable, V any] struct {
	keys []K
	data map[K]V
}

func newOrderedYAMLMap[K comparable, V any](keys []K, values []V) (*orderedYAMLMap[K, V], error) {
	if len(keys) != len(values) {
		return nil, fmt.Errorf("keys and values must have the same length")
	}

	om := &orderedYAMLMap[K, V]{
		keys: make([]K, len(keys)),
		data: make(map[K]V, len(keys)),
	}

	copy(om.keys, keys)

	for i := range keys {
		om.data[keys[i]] = values[i]
	}

	return om, nil
}

func (om *orderedYAMLMap[K, V]) Items() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, key := range om.keys {
			value, ok := om.data[key]
			if !ok {
				continue
			}
			if !yield(key, value) {
				return
			}
		}
	}
}

func (om *orderedYAMLMap[K, V]) Get(key K) (V, bool) {
	value, ok := om.data[key]
	return value, ok
}

func (self *orderedYAMLMap[K, V]) Merge(other *orderedYAMLMap[K, V]) *orderedYAMLMap[K, V] {
	merged := &orderedYAMLMap[K, V]{
		keys: make([]K, 0, len(self.keys)+len(other.keys)),
		data: make(map[K]V, len(self.data)+len(other.data)),
	}

	merged.keys = append(merged.keys, self.keys...)
	maps.Copy(merged.data, self.data)

	for _, key := range other.keys {
		if _, exists := self.data[key]; !exists {
			merged.keys = append(merged.keys, key)
		}
	}
	maps.Copy(merged.data, other.data)

	return merged
}

func (om *orderedYAMLMap[K, V]) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("orderedMap: expected mapping node, got %d", node.Kind)
	}

	if len(node.Content)%2 != 0 {
		return fmt.Errorf("orderedMap: expected even number of content items, got %d", len(node.Content))
	}

	om.keys = make([]K, len(node.Content)/2)
	om.data = make(map[K]V, len(node.Content)/2)

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		var key K
		if err := keyNode.Decode(&key); err != nil {
			return fmt.Errorf("orderedMap: decoding key: %v", err)
		}

		if _, ok := om.data[key]; ok {
			return fmt.Errorf("orderedMap: duplicate key %v", key)
		}

		var value V
		if err := valueNode.Decode(&value); err != nil {
			return fmt.Errorf("orderedMap: decoding value: %v", err)
		}

		(*om).keys[i/2] = key
		(*om).data[key] = value
	}

	return nil
}
