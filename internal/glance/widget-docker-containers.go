package glance

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"
)

var dockerContainersWidgetTemplate = mustParseTemplate("docker-containers.html", "widget-base.html")

type dockerContainersWidget struct {
	widgetBase    `yaml:",inline"`
	HideByDefault bool                `yaml:"hide-by-default"`
	SockPath      string              `yaml:"sock-path"`
	Containers    dockerContainerList `yaml:"-"`
}

func (widget *dockerContainersWidget) initialize() error {
	widget.withTitle("Docker Containers").withCacheDuration(1 * time.Minute)

	if widget.SockPath == "" {
		widget.SockPath = "/var/run/docker.sock"
	}

	return nil
}

func (widget *dockerContainersWidget) update(ctx context.Context) {
	containers, err := fetchDockerContainers(widget.SockPath, widget.HideByDefault)
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
	Title       string
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

		return strings.ToLower(containers[a].Title) < strings.ToLower(containers[b].Title)
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

func fetchDockerContainers(socketPath string, hideByDefault bool) (dockerContainerList, error) {
	containers, err := fetchAllDockerContainersFromSock(socketPath)
	if err != nil {
		return nil, fmt.Errorf("fetching containers: %w", err)
	}

	containers, children := groupDockerContainerChildren(containers, hideByDefault)
	dockerContainers := make(dockerContainerList, 0, len(containers))

	for i := range containers {
		container := &containers[i]

		dc := dockerContainer{
			Title:       deriveDockerContainerTitle(container),
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
						Title:     deriveDockerContainerTitle(child),
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

func deriveDockerContainerTitle(container *dockerContainerJsonResponse) string {
	if v := container.Labels.getOrDefault(dockerContainerLabelName, ""); v != "" {
		return v
	}

	return strings.TrimLeft(itemAtIndexOrDefault(container.Names, 0, "n/a"), "/")
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

func fetchAllDockerContainersFromSock(socketPath string) ([]dockerContainerJsonResponse, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	request, err := http.NewRequest("GET", "http://docker/containers/json?all=true", nil)
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

	return containers, nil
}
