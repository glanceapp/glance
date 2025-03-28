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
	Mode          string              `yaml:"mode"`
	Containers    dockerContainerList `yaml:"-"`
}

func (widget *dockerContainersWidget) initialize() error {
	widget.withTitle("Docker Containers").withCacheDuration(1 * time.Minute)

	if widget.SockPath == "" {
		widget.SockPath = "/var/run/docker.sock"
	}

	widget.Mode = strings.ToLower(widget.Mode)
	if widget.Mode == "" {
		widget.Mode = dockerContainerModeStandalone
	} else if !dockerContainerValidModes[widget.Mode] {
		validModes := make([]string, 0, len(dockerContainerValidModes))
		for key := range dockerContainerValidModes {
			validModes = append(validModes, key)
		}

		return fmt.Errorf(
			"invalid mode: %q, must be one of %q",
			widget.Mode, strings.Join(validModes, ","),
		)
	}

	return nil
}

func (widget *dockerContainersWidget) update(ctx context.Context) {
	containers, err := fetchDockerContainers(widget.SockPath, widget.HideByDefault, widget.Mode)
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
	dockerContainerModeStandalone = "standalone"
	dockerContainerModeSwarm      = "swarm"
)

var dockerContainerValidModes = map[string]bool{
	dockerContainerModeStandalone: true,
	dockerContainerModeSwarm:      true,
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
	dockerContainerStateIconOK      = "ok"
	dockerContainerStateIconPending = "pending"
	dockerContainerStateIconPaused  = "paused"
	dockerContainerStateIconWarn    = "warn"
	dockerContainerStateIconOther   = "other"
)

var dockerContainerStateIconPriorities = map[string]int{
	dockerContainerStateIconWarn:    0,
	dockerContainerStateIconOther:   1,
	dockerContainerStateIconPending: 2,
	dockerContainerStateIconPaused:  3,
	dockerContainerStateIconOK:      4,
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

type swarmNodeJsonResponse struct {
	ID        string `json:"ID"`
	CreatedAt string `json:"CreatedAt"`
	UpdatedAt string `json:"UpdatedAt"`
	Spec      struct {
		Role         string `json:"Role"`
		Availability string `json:"Availability"`
	} `json:"Spec"`
	Status struct {
		State string `json:"State"`
		Addr  string `json:"Addr"`
	} `json:"Status"`
	ManagerStatus struct {
		Leader       bool   `json:"Leader"`
		Reachability string `json:"Reachability"`
		Addr         string `json:"Addr"`
	} `json:"ManagerStatus"`
}

type swarmServiceJsonResponse struct {
	ID        string `json:"ID"`
	CreatedAt string `json:"CreatedAt"`
	UpdatedAt string `json:"UpdatedAt"`
	Spec      struct {
		Name         string                `json:"Name"`
		Labels       dockerContainerLabels `json:"Labels"`
		TaskTemplate struct {
			ContainerSpec struct {
				Image string `json:"Image"`
			} `json:"ContainerSpec"`
		} `json:"TaskTemplate"`
		Mode struct {
			Global     *struct{} `json:"Global,omitempty"`
			Replicated *struct {
				Replicas int `json:"Replicas"`
			} `json:"Replicated,omitempty"`
		} `json:"Mode"`
	} `json:"Spec"`
}

type swarmTaskJsonResponse struct {
	ID           string `json:"ID"`
	ServiceID    string `json:"ServiceID"`
	CreatedAt    string `json:"CreatedAt"`
	UpdatedAt    string `json:"UpdatedAt"`
	DesiredState string `json:"DesiredState"`
	NodeID       string `json:"NodeID"`
	Status       struct {
		Timestamp string `json:"Timestamp"`
		State     string `json:"State"`
		Message   string `json:"Message"`
	} `json:"Status"`
}

type swarmTaskJsonList []swarmTaskJsonResponse

func (tasks swarmTaskJsonList) sortByStatusUpdatedThenCreated() {
	sort.SliceStable(tasks, func(a, b int) bool {
		timestampA, errA := time.Parse(time.RFC3339, tasks[a].Status.Timestamp)
		timestampB, errB := time.Parse(time.RFC3339, tasks[b].Status.Timestamp)

		if errA == nil && errB == nil {
			return timestampA.After(timestampB)
		}

		updatedA, errA := time.Parse(time.RFC3339, tasks[a].UpdatedAt)
		updatedB, errB := time.Parse(time.RFC3339, tasks[b].UpdatedAt)

		if errA == nil && errB == nil {
			return updatedA.After(updatedB)
		}

		createdA, errA := time.Parse(time.RFC3339, tasks[a].CreatedAt)
		createdB, errB := time.Parse(time.RFC3339, tasks[b].CreatedAt)

		if errA == nil && errB == nil {
			return createdA.After(createdB)
		}

		return false
	})
}

type swarmExtendedService struct {
	*swarmServiceJsonResponse `json:"Service"`
	State                     string `json:"State"`
	Status                    string `json:"Status"`
}

type swarmExtendedServiceList []swarmExtendedService

func (services swarmExtendedServiceList) toContainerJsonList() []dockerContainerJsonResponse {
	containers := make([]dockerContainerJsonResponse, 0, len(services))

	for i := range services {
		service := &services[i]
		container := dockerContainerJsonResponse{
			Names:  []string{service.Spec.Name},
			Image:  service.Spec.TaskTemplate.ContainerSpec.Image,
			State:  service.State,
			Status: service.Status,
			Labels: service.Spec.Labels,
		}

		containers = append(containers, container)
	}

	return containers
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
	case "running", "complete":
		return dockerContainerStateIconOK
	case "paused":
		return dockerContainerStateIconPaused
	case "pending":
		return dockerContainerStateIconPending
	case "exited", "unhealthy", "dead":
		return dockerContainerStateIconWarn
	default:
		return dockerContainerStateIconOther
	}
}

func fetchDockerContainers(socketPath string, hideByDefault bool, mode string) (dockerContainerList, error) {
	containers, err := fetchContainersByMode(socketPath, mode)
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

func fetchContainersByMode(socketPath string, mode string) ([]dockerContainerJsonResponse, error) {
	switch mode {
	case dockerContainerModeSwarm:
		return getSwarmContainers(socketPath)
	default:
		return getDockerContainers(socketPath)
	}
}

func getSwarmContainers(socketPath string) ([]dockerContainerJsonResponse, error) {
	svcs, err := fetchAllSwarmServicesFromSock(socketPath)
	if err != nil {
		return nil, fmt.Errorf("fetching swarm services: %w", err)
	}

	nodes, err := fetchAllSwarmNodesFromSock(socketPath)
	if err != nil {
		return nil, fmt.Errorf("fetching swarm nodes: %w", err)
	}

	tasks, err := fetchAllSwarmTasksFromSock(socketPath)
	if err != nil {
		return nil, fmt.Errorf("fetching swarm tasks: %w", err)
	}

	services := extendSwarmServices(svcs, nodes, tasks)
	containers := services.toContainerJsonList()
	return containers, nil
}

func getDockerContainers(socketPath string) ([]dockerContainerJsonResponse, error) {
	containers, err := fetchAllDockerContainersFromSock(socketPath)
	if err != nil {
		return nil, fmt.Errorf("fetching docker containers: %w", err)
	}

	return containers, nil
}

func fetchAllDockerContainersFromSock(socketPath string) ([]dockerContainerJsonResponse, error) {
	var containers []dockerContainerJsonResponse
	if err := fetchFromSock(socketPath, "http://docker/containers/json?all=true", &containers); err != nil {
		return nil, err
	}

	return containers, nil
}

func fetchAllSwarmNodesFromSock(socketPath string) ([]swarmNodeJsonResponse, error) {
	var nodes []swarmNodeJsonResponse
	if err := fetchFromSock(socketPath, "http://docker/nodes", &nodes); err != nil {
		return nil, err
	}

	return nodes, nil
}

func fetchAllSwarmServicesFromSock(socketPath string) ([]swarmServiceJsonResponse, error) {
	var services []swarmServiceJsonResponse
	if err := fetchFromSock(socketPath, "http://docker/services", &services); err != nil {
		return nil, err
	}

	return services, nil
}

func fetchAllSwarmTasksFromSock(socketPath string) (swarmTaskJsonList, error) {
	var tasks []swarmTaskJsonResponse
	if err := fetchFromSock(socketPath, "http://docker/tasks", &tasks); err != nil {
		return nil, err
	}

	return tasks, nil
}

func extendSwarmServices(
	services []swarmServiceJsonResponse,
	nodes []swarmNodeJsonResponse,
	tasks swarmTaskJsonList,
) swarmExtendedServiceList {
	tasks.sortByStatusUpdatedThenCreated()
	tasksByService := make(map[string][]swarmTaskJsonResponse)
	for _, task := range tasks {
		tasksByService[task.ServiceID] = append(tasksByService[task.ServiceID], task)
	}

	servicesExtended := make(swarmExtendedServiceList, 0, len(services))
	for i := range services {
		service := &services[i]
		serviceTasks := tasksByService[service.ID]
		state, statusMsg := deriveServiceStatus(service, nodes, serviceTasks)
		servicesExtended = append(servicesExtended, swarmExtendedService{
			swarmServiceJsonResponse: service,
			State:                    state,
			Status:                   statusMsg,
		})
	}

	return servicesExtended
}

func deriveServiceStatus(
	service *swarmServiceJsonResponse,
	nodes []swarmNodeJsonResponse,
	tasks []swarmTaskJsonResponse,
) (string, string) {
	expectedReplicas, isGlobal := calculateExpectedReplicas(service, nodes)
	taskStateCounts, tasksForErrorMsg := aggregateTaskStates(tasks)
	totalTasks := len(tasks)

	if totalTasks == 0 && expectedReplicas > 0 {
		return "pending", "Service created but no tasks are assigned yet"
	} else if expectedReplicas == 0 {
		tasksUp := taskStateCounts["running"] + taskStateCounts["pending"]
		if tasksUp > 0 {
			return "pending", fmt.Sprintf("Service scaling down (%d tasks terminating)", tasksUp)
		}

		return "complete", "Service scaled to 0 replicas"
	} else if complete := taskStateCounts["complete"]; complete == expectedReplicas {
		return "complete", fmt.Sprintf("Service tasks completed (%d tasks)", complete)
	} else if running := taskStateCounts["running"]; running == expectedReplicas {
		statusMsg := fmt.Sprintf("Service running (%d/%d replicas)", running, expectedReplicas)
		if isGlobal {
			statusMsg = fmt.Sprintf("Service running globally (%d/%d nodes)", running, expectedReplicas)
		}

		return "running", statusMsg
	} else if pending := taskStateCounts["pending"]; pending > 0 {
		statusMsg := fmt.Sprintf("Service pending (%d/%d replicas running, %d pending)", running, expectedReplicas, pending)
		if isGlobal {
			statusMsg = fmt.Sprintf("Service pending globally (%d/%d nodes running, %d pending)", running, expectedReplicas, pending)
		}

		return "pending", statusMsg
	} else if unhealthy := taskStateCounts["unhealthy"]; unhealthy > 0 {
		state := "unhealthy"
		statusMsg := fmt.Sprintf("Service degraded (%d tasks unhealthy)", unhealthy)
		if len(tasksForErrorMsg) > 0 {
			details := combineTaskStatusMessages(tasksForErrorMsg, 3)
			if details != "" {
				statusMsg += ": " + details
			}
		}

		if taskStateCounts["running"] > 0 {
			state = "warn"
			statusMsg = fmt.Sprintf("%s, %d/%d healthy", statusMsg, taskStateCounts["running"], expectedReplicas)
		}

		return state, statusMsg
	} else if other := taskStateCounts["other"]; other > 0 {
		return "other", fmt.Sprintf("Service in an unknown state (%d tasks in unexpected states)", other)
	}

	return "other", "Service in an uknown state"
}

func calculateExpectedReplicas(
	service *swarmServiceJsonResponse,
	nodes []swarmNodeJsonResponse,
) (int, bool) {
	expectedReplicas := 0
	isGlobal := service.Spec.Mode.Global != nil

	if isGlobal {
		availableNodes := 0
		for _, node := range nodes {
			if node.Status.State == "ready" && node.Spec.Availability == "active" {
				availableNodes++
			}
		}

		expectedReplicas = availableNodes
	} else if service.Spec.Mode.Replicated != nil && service.Spec.Mode.Replicated.Replicas > 0 {
		expectedReplicas = service.Spec.Mode.Replicated.Replicas
	}

	return expectedReplicas, isGlobal
}

func aggregateTaskStates(
	tasks []swarmTaskJsonResponse,
) (map[string]int, []swarmTaskJsonResponse) {
	tasksForErrorMsg := make([]swarmTaskJsonResponse, 0)
	taskStateCounts := make(map[string]int, 0)
	for _, task := range tasks {
		if task.DesiredState == task.Status.State {
			taskStateCounts[task.Status.State]++
		} else {
			switch task.Status.State {
			case "running", "new", "pending", "assigned", "accepted", "preparing", "starting", "ready":
				taskStateCounts["pending"]++
			case "complete", "failed", "rejected", "shutdown", "orphaned", "remove":
				taskStateCounts["unhealthy"]++
				tasksForErrorMsg = append(tasksForErrorMsg, task)
			default:
				taskStateCounts["other"]++
			}
		}
	}

	return taskStateCounts, tasksForErrorMsg
}

func combineTaskStatusMessages(tasks []swarmTaskJsonResponse, maxMessages int) string {
	if len(tasks) == 0 || maxMessages <= 0 {
		return ""
	}

	messageCount := min(len(tasks), maxMessages)
	messages := make([]string, 0, messageCount)

	for i := range messageCount {
		taskMessage := tasks[i].Status.Message
		if taskMessage != "" {
			id, _ := limitStringLength(tasks[i].ID, 12)
			messages = append(messages, fmt.Sprintf("Task %s: %s", id, taskMessage))
		}
	}

	if len(messages) == 0 {
		return ""
	}

	if len(tasks) > maxMessages {
		messages = append(messages, fmt.Sprintf("...and %d more tasks", len(tasks)-maxMessages))
	}

	return strings.Join(messages, "; ")
}

func fetchFromSock[T any](socketPath string, path string, target T) error {
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	request, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("sending request to socket: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("non-200 response status: %s", response.Status)
	}

	if err := json.NewDecoder(response.Body).Decode(&target); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	return nil
}
