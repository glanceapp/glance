package glance

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

var dockerContainersWidgetTemplate = mustParseTemplate("docker-containers.html", "widget-base.html")

type dockerContainersWidget struct {
	widgetBase           `yaml:",inline"`
	HideByDefault        bool                         `yaml:"hide-by-default"`
	RunningOnly          bool                         `yaml:"running-only"`
	Category             string                       `yaml:"category"`
	SockPath             string                       `yaml:"sock-path"`
	FormatContainerNames bool                         `yaml:"format-container-names"`
	Containers           dockerContainerList          `yaml:"-"`
	LabelOverrides       map[string]map[string]string `yaml:"containers"`
}

func (widget *dockerContainersWidget) initialize() error {
	widget.withTitle("Docker Containers").withCacheDuration(1 * time.Minute)

	if widget.SockPath == "" {
		widget.SockPath = "/var/run/docker.sock"
	}

	return nil
}

func (widget *dockerContainersWidget) update(ctx context.Context) {
	containers, err := fetchDockerContainers(
		widget.SockPath,
		widget.HideByDefault,
		widget.Category,
		widget.RunningOnly,
		widget.FormatContainerNames,
		widget.LabelOverrides,
	)
	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	containers.sortByStateIconThenTitle()
	widget.Containers = containers
}

func (widget *dockerContainersWidget) Render() template.HTML {
	return widget.renderTemplate(widget, dockerContainersWidgetTemplate)
}

const (
	dockerContainerLabelHide        = "glance.hide"
	dockerContainerLabelName        = "glance.name"
	dockerContainerLabelURL         = "glance.url"
	dockerContainerLabelDescription = "glance.description"
	dockerContainerLabelSameTab     = "glance.same-tab"
	dockerContainerLabelIcon        = "glance.icon"
	dockerContainerLabelID          = "glance.id"
	dockerContainerLabelParent      = "glance.parent"
	dockerContainerLabelCategory    = "glance.category"
)

const (
	dockerContainerStateIconOK     = "ok"
	dockerContainerStateIconPaused = "paused"
	dockerContainerStateIconWarn   = "warn"
	dockerContainerStateIconOther  = "other"
)

var dockerContainerStateIconPriorities = map[string]int{
	dockerContainerStateIconWarn:   0,
	dockerContainerStateIconOther:  1,
	dockerContainerStateIconPaused: 2,
	dockerContainerStateIconOK:     3,
}

type dockerContainerJsonResponse struct {
	Names  []string              `json:"Names"`
	Image  string                `json:"Image"`
	State  string                `json:"State"`
	Status string                `json:"Status"`
	Labels dockerContainerLabels `json:"Labels"`
}

type dockerContainerLabels map[string]string

func (l *dockerContainerLabels) getOrDefault(label, def string) string {
	if l == nil {
		return def
	}

	v, ok := (*l)[label]
	if !ok {
		return def
	}

	if v == "" {
		return def
	}

	return v
}

type dockerContainer struct {
	Name        string
	URL         string
	SameTab     bool
	Image       string
	State       string
	StateText   string
	StateIcon   string
	Description string
	Icon        customIconField
	Children    dockerContainerList
}

type dockerContainerList []dockerContainer

func (containers dockerContainerList) sortByStateIconThenTitle() {
	p := &dockerContainerStateIconPriorities

	sort.SliceStable(containers, func(a, b int) bool {
		if containers[a].StateIcon != containers[b].StateIcon {
			return (*p)[containers[a].StateIcon] < (*p)[containers[b].StateIcon]
		}

		return strings.ToLower(containers[a].Name) < strings.ToLower(containers[b].Name)
	})
}

func dockerContainerStateToStateIcon(state string) string {
	switch state {
	case "running":
		return dockerContainerStateIconOK
	case "paused":
		return dockerContainerStateIconPaused
	case "exited", "unhealthy", "dead":
		return dockerContainerStateIconWarn
	default:
		return dockerContainerStateIconOther
	}
}

func fetchDockerContainers(
	socketPath string,
	hideByDefault bool,
	category string,
	runningOnly bool,
	formatNames bool,
	labelOverrides map[string]map[string]string,
) (dockerContainerList, error) {
	containers, err := fetchDockerContainersFromSource(socketPath, category, runningOnly, labelOverrides)
	if err != nil {
		return nil, fmt.Errorf("fetching containers: %w", err)
	}

	containers, children := groupDockerContainerChildren(containers, hideByDefault)
	dockerContainers := make(dockerContainerList, 0, len(containers))

	for i := range containers {
		container := &containers[i]

		dc := dockerContainer{
			Name:        deriveDockerContainerName(container, formatNames),
			URL:         container.Labels.getOrDefault(dockerContainerLabelURL, ""),
			Description: container.Labels.getOrDefault(dockerContainerLabelDescription, ""),
			SameTab:     stringToBool(container.Labels.getOrDefault(dockerContainerLabelSameTab, "false")),
			Image:       container.Image,
			State:       strings.ToLower(container.State),
			StateText:   strings.ToLower(container.Status),
			Icon:        newCustomIconField(container.Labels.getOrDefault(dockerContainerLabelIcon, "si:docker")),
		}

		if idValue := container.Labels.getOrDefault(dockerContainerLabelID, ""); idValue != "" {
			if children, ok := children[idValue]; ok {
				for i := range children {
					child := &children[i]
					dc.Children = append(dc.Children, dockerContainer{
						Name:      deriveDockerContainerName(child, formatNames),
						StateText: child.Status,
						StateIcon: dockerContainerStateToStateIcon(strings.ToLower(child.State)),
					})
				}
			}
		}

		dc.Children.sortByStateIconThenTitle()

		stateIconSupersededByChild := false
		for i := range dc.Children {
			if dc.Children[i].StateIcon == dockerContainerStateIconWarn {
				dc.StateIcon = dockerContainerStateIconWarn
				stateIconSupersededByChild = true
				break
			}
		}
		if !stateIconSupersededByChild {
			dc.StateIcon = dockerContainerStateToStateIcon(dc.State)
		}

		dockerContainers = append(dockerContainers, dc)
	}

	return dockerContainers, nil
}

func deriveDockerContainerName(container *dockerContainerJsonResponse, formatNames bool) string {
	if v := container.Labels.getOrDefault(dockerContainerLabelName, ""); v != "" {
		return v
	}

	if len(container.Names) == 0 || container.Names[0] == "" {
		return "n/a"
	}

	name := strings.TrimLeft(container.Names[0], "/")

	if formatNames {
		name = strings.ReplaceAll(name, "_", " ")
		name = strings.ReplaceAll(name, "-", " ")

		words := strings.Split(name, " ")
		for i := range words {
			if len(words[i]) > 0 {
				words[i] = strings.ToUpper(words[i][:1]) + words[i][1:]
			}
		}
		name = strings.Join(words, " ")
	}

	return name
}

func groupDockerContainerChildren(
	containers []dockerContainerJsonResponse,
	hideByDefault bool,
) (
	[]dockerContainerJsonResponse,
	map[string][]dockerContainerJsonResponse,
) {
	parents := make([]dockerContainerJsonResponse, 0, len(containers))
	children := make(map[string][]dockerContainerJsonResponse)

	for i := range containers {
		container := &containers[i]

		if isDockerContainerHidden(container, hideByDefault) {
			continue
		}

		isParent := container.Labels.getOrDefault(dockerContainerLabelID, "") != ""
		parent := container.Labels.getOrDefault(dockerContainerLabelParent, "")

		if !isParent && parent != "" {
			children[parent] = append(children[parent], *container)
		} else {
			parents = append(parents, *container)
		}
	}

	return parents, children
}

func isDockerContainerHidden(container *dockerContainerJsonResponse, hideByDefault bool) bool {
	if v := container.Labels.getOrDefault(dockerContainerLabelHide, ""); v != "" {
		return stringToBool(v)
	}

	return hideByDefault
}


func fetchDockerContainersFromSource(
	source string,
	category string,
	runningOnly bool,
	labelOverrides map[string]map[string]string,
) ([]dockerContainerJsonResponse, error) {
	var hostname string

	var client *http.Client
	if strings.HasPrefix(source, "tcp://") || strings.HasPrefix(source, "http://") {
		client = &http.Client{}
		parsed, err := url.Parse(source)
		if err != nil {
			return nil, fmt.Errorf("parsing URL: %w", err)
		}

		port := parsed.Port()
		if port == "" {
			port = "80"
		}

		hostname = parsed.Hostname() + ":" + port
	} else {
		hostname = "docker"
		client = &http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", source)
				},
			},
		}
	}


	fetchAll := ternary(runningOnly, "false", "true")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, "GET", "http://"+hostname+"/containers/json?all="+fetchAll, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("sending request to socket: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 response status: %s", response.Status)
	}

	var containers []dockerContainerJsonResponse
	if err := json.NewDecoder(response.Body).Decode(&containers); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	for i := range containers {
		container := &containers[i]
		name := strings.TrimLeft(itemAtIndexOrDefault(container.Names, 0, ""), "/")

		if name == "" {
			continue
		}

		overrides, ok := labelOverrides[name]
		if !ok {
			continue
		}

		if container.Labels == nil {
			container.Labels = make(dockerContainerLabels)
		}

		for label, value := range overrides {
			container.Labels["glance."+label] = value
		}
	}

	// We have to filter here instead of using the `filters` parameter of Docker's API
	// because the user may define a category override within their config
	if category != "" {
		filtered := make([]dockerContainerJsonResponse, 0, len(containers))

		for i := range containers {
			container := &containers[i]

			if container.Labels.getOrDefault(dockerContainerLabelCategory, "") == category {
				filtered = append(filtered, *container)
			}
		}

		containers = filtered
	}

	return containers, nil
}
