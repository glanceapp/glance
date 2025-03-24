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

var swarmServicesWidgetTemplate = mustParseTemplate("swarm-services.html", "widget-base.html")

type swarmServicesWidget struct {
	widgetBase    `yaml:",inline"`
	HideByDefault bool             `yaml:"hide-by-default"`
	SockPath      string           `yaml:"sock-path"`
	Services      swarmServiceList `yaml:"-"`
}

func (widget *swarmServicesWidget) initialize() error {
	widget.withTitle("Swarm Services").withCacheDuration(1 * time.Minute)

	if widget.SockPath == "" {
		widget.SockPath = "/var/run/docker.sock"
	}

	return nil
}

func (widget *swarmServicesWidget) update(ctx context.Context) {
	services, err := fetchSwarmServices(widget.SockPath, widget.HideByDefault)
	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	services.sortByStateIconThenTitle()
	widget.Services = services
}

func (widget *swarmServicesWidget) Render() template.HTML {
	return widget.renderTemplate(widget, swarmServicesWidgetTemplate)
}

const (
	swarmServicesLabelHide       = "glance.hide"
	swarmServiceLabelName        = "glance.name"
	swarmServiceLabelURL         = "glance.url"
	swarmServiceLabelDescription = "glance.description"
	swarmServiceLabelSameTab     = "glance.same-tab"
	swarmServiceLabelIcon        = "glance.icon"
	swarmServiceLabelID          = "glance.id"
	swarmServiceLabelParent      = "glance.parent"
)

const (
	swarmServiceStateIconOK      = "ok"
	swarmServiceStateIconPending = "pending"
	swarmServiceStateIconWarn    = "warn"
	swarmServiceStateIconOther   = "other"
)

var swarmServiceStateIconPriorities = map[string]int{
	swarmServiceStateIconWarn:    0,
	swarmServiceStateIconOther:   1,
	swarmServiceStateIconPending: 2,
	swarmServiceStateIconOK:      3,
}

type swarmServiceJsonResponse struct {
	ID   string           `json:"ID"`
	Spec swarmServiceSpec `json:"Spec"`
}

type swarmServiceSpec struct {
	Name         string                   `json:"Name"`
	Labels       swarmServiceLabels       `json:"Labels"`
	TaskTemplate swarmServiceTaskTemplate `json:"TaskTemplate"`
}

type swarmServiceLabels map[string]string

type swarmServiceTaskTemplate struct {
	ContainerSpec swarmServiceContainerSpec `json:"ContainerSpec"`
}

type swarmServiceContainerSpec struct {
	Image string `json:"Image"`
}

type swarmTaskJsonResponse struct {
	ID        string `json:"ID"`
	ServiceID string `json:"ServiceID"`
	Status    struct {
		State   string `json:"State"`
		Message string `json:"Message"`
	} `json:"Status"`
}

type swarmExtendedService struct {
	*swarmServiceJsonResponse `json:"Service"`
	State                     string `json:"State"`
	Status                    string `json:"Status"`
}

func (l *swarmServiceLabels) getOrDefault(label, def string) string {
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

type swarmService struct {
	Title       string
	URL         string
	SameTab     bool
	Image       string
	State       string
	StateText   string
	StateIcon   string
	Description string
	Icon        customIconField
	Children    swarmServiceList
}

type swarmServiceList []swarmService

func (services swarmServiceList) sortByStateIconThenTitle() {
	p := &swarmServiceStateIconPriorities

	sort.SliceStable(services, func(a, b int) bool {
		if services[a].StateIcon != services[b].StateIcon {
			return (*p)[services[a].StateIcon] < (*p)[services[b].StateIcon]
		}

		return strings.ToLower(services[a].Title) < strings.ToLower(services[b].Title)
	})
}

func swarmServiceStateToStateIcon(state string) string {
	switch state {
	case "running", "complete":
		return swarmServiceStateIconOK
	case "new", "pending", "assigned", "accepted", "ready", "preparing", "starting":
		return swarmServiceStateIconPending
	case "failed", "shutdown", "rejected", "orphaned", "remove":
		return swarmServiceStateIconWarn
	default:
		return swarmServiceStateIconOther
	}
}

func fetchSwarmServices(socketPath string, hideByDefault bool) (swarmServiceList, error) {
	svcs, err := fetchAllSwarmServicesFromSock(socketPath)
	if err != nil {
		return nil, fmt.Errorf("fetching services: %w", err)
	}

	tasks, err := fetchAllSwarmTasksFromSock(socketPath)
	if err != nil {
		return nil, fmt.Errorf("fetching tasks: %w", err)
	}

	services := groupSwarmServiceTasks(svcs, tasks)
	services, children := groupSwarmServiceChildren(services, hideByDefault)

	swarmServices := make(swarmServiceList, 0, len(services))

	for i := range services {
		service := &services[i]

		dc := swarmService{
			Title:       deriveSwarmServiceTitle(service),
			URL:         service.Spec.Labels.getOrDefault(swarmServiceLabelURL, ""),
			Description: service.Spec.Labels.getOrDefault(swarmServiceLabelDescription, ""),
			SameTab:     stringToBool(service.Spec.Labels.getOrDefault(swarmServiceLabelSameTab, "false")),
			Image:       service.Spec.TaskTemplate.ContainerSpec.Image,
			State:       strings.ToLower(service.State),
			StateText:   strings.ToLower(service.Status),
			Icon:        newCustomIconField(service.Spec.Labels.getOrDefault(swarmServiceLabelIcon, "si:docker")),
		}

		if idValue := service.Spec.Labels.getOrDefault(swarmServiceLabelID, ""); idValue != "" {
			if children, ok := children[idValue]; ok {
				for i := range children {
					child := &children[i]
					dc.Children = append(dc.Children, swarmService{
						Title:     deriveSwarmServiceTitle(child),
						StateText: child.Status,
						StateIcon: swarmServiceStateToStateIcon(strings.ToLower(child.State)),
					})
				}
			}
		}

		dc.Children.sortByStateIconThenTitle()

		stateIconSupersededByChild := false
		for i := range dc.Children {
			if dc.Children[i].StateIcon == swarmServiceStateIconWarn {
				dc.StateIcon = swarmServiceStateIconWarn
				stateIconSupersededByChild = true
				break
			}
		}
		if !stateIconSupersededByChild {
			dc.StateIcon = swarmServiceStateToStateIcon(dc.State)
		}

		swarmServices = append(swarmServices, dc)
	}

	return swarmServices, nil
}

func deriveSwarmServiceTitle(service *swarmExtendedService) string {
	if v := service.Spec.Labels.getOrDefault(swarmServiceLabelName, ""); v != "" {
		return v
	}

	return strings.TrimLeft(service.Spec.Name, "/")
}

func groupSwarmServiceChildren(
	services []swarmExtendedService,
	hideByDefault bool,
) (
	[]swarmExtendedService,
	map[string][]swarmExtendedService,
) {
	parents := make([]swarmExtendedService, 0, len(services))
	children := make(map[string][]swarmExtendedService)

	for i := range services {
		service := &services[i]

		if isSwarmServiceHidden(service, hideByDefault) {
			continue
		}

		isParent := service.Spec.Labels.getOrDefault(swarmServiceLabelID, "") != ""
		parent := service.Spec.Labels.getOrDefault(swarmServiceLabelParent, "")

		if !isParent && parent != "" {
			children[parent] = append(children[parent], *service)
		} else {
			parents = append(parents, *service)
		}
	}

	return parents, children
}

func groupSwarmServiceTasks(
	services []swarmServiceJsonResponse,
	tasks []swarmTaskJsonResponse,
) []swarmExtendedService {
	servicesWithTasks := make([]swarmExtendedService, 0, len(services))

	for i := range services {
		service := &services[i]
		extended := swarmExtendedService{service, "unknown", "no tasks found"}

		for _, task := range tasks {
			// TODO: Is there a better way to "prioritize" running tasks over shutdown?
			if task.Status.State == "running" {
				extended.State = task.Status.State
				extended.Status = task.Status.Message
				break
			}

			if task.ServiceID == service.ID {
				extended.State = task.Status.State
				extended.Status = task.Status.Message
			}
		}

		servicesWithTasks = append(servicesWithTasks, extended)
	}

	return servicesWithTasks
}

func isSwarmServiceHidden(service *swarmExtendedService, hideByDefault bool) bool {
	if v := service.Spec.Labels.getOrDefault(swarmServicesLabelHide, ""); v != "" {
		return stringToBool(v)
	}

	return hideByDefault
}

func fetchAllSwarmServicesFromSock(socketPath string) ([]swarmServiceJsonResponse, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	request, err := http.NewRequest("GET", "http://docker/services", nil)
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

	var services []swarmServiceJsonResponse
	if err := json.NewDecoder(response.Body).Decode(&services); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return services, nil
}

func fetchAllSwarmTasksFromSock(socketPath string) ([]swarmTaskJsonResponse, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	request, err := http.NewRequest("GET", "http://docker/tasks", nil)
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

	var tasks []swarmTaskJsonResponse
	if err := json.NewDecoder(response.Body).Decode(&tasks); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return tasks, nil
}
