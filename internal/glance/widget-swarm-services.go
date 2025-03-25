package glance

import (
	"fmt"
	"strings"
)

type swarmServiceJsonResponse struct {
	ID   string           `json:"ID"`
	Spec swarmServiceSpec `json:"Spec"`
}

type swarmTaskJsonResponse struct {
	ID        string          `json:"ID"`
	ServiceID string          `json:"ServiceID"`
	Status    swarmTaskStatus `json:"Status"`
}

type swarmServiceSpec struct {
	Name         string                `json:"Name"`
	Labels       dockerContainerLabels `json:"Labels"`
	TaskTemplate swarmTaskTemplate     `json:"TaskTemplate"`
}

type swarmTaskTemplate struct {
	ContainerSpec swarmContainerSpec `json:"ContainerSpec"`
}

type swarmContainerSpec struct {
	Image string `json:"Image"`
}

type swarmTaskStatus struct {
	State   string `json:"State"`
	Message string `json:"Message"`
}

type swarmExtendedService struct {
	*swarmServiceJsonResponse `json:"Service"`
	State                     string `json:"State"`
	Status                    string `json:"Status"`
}

func fetchSwarmServices(socketPath string, hideByDefault bool) (dockerContainerList, error) {
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

	swarmServices := make(dockerContainerList, 0, len(services))

	for i := range services {
		service := &services[i]

		dc := dockerContainer{
			Title:       deriveDockerContainerTitle(&service.Spec.Labels, service.Spec.Name),
			URL:         service.Spec.Labels.getOrDefault(dockerContainerLabelURL, ""),
			Description: service.Spec.Labels.getOrDefault(dockerContainerLabelDescription, ""),
			SameTab:     stringToBool(service.Spec.Labels.getOrDefault(dockerContainerLabelSameTab, "false")),
			Image:       service.Spec.TaskTemplate.ContainerSpec.Image,
			State:       strings.ToLower(service.State),
			StateText:   strings.ToLower(service.Status),
			Icon:        newCustomIconField(service.Spec.Labels.getOrDefault(dockerContainerLabelIcon, "si:docker")),
		}

		if idValue := service.Spec.Labels.getOrDefault(dockerContainerLabelID, ""); idValue != "" {
			if children, ok := children[idValue]; ok {
				for i := range children {
					child := &children[i]
					dc.Children = append(dc.Children, dockerContainer{
						Title:     deriveDockerContainerTitle(&child.Spec.Labels, child.Spec.Name),
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

		swarmServices = append(swarmServices, dc)
	}

	return swarmServices, nil
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

		if isDockerContainerHidden(&service.Spec.Labels, hideByDefault) {
			continue
		}

		isParent := service.Spec.Labels.getOrDefault(dockerContainerLabelID, "") != ""
		parent := service.Spec.Labels.getOrDefault(dockerContainerLabelParent, "")

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

func fetchAllSwarmServicesFromSock(socketPath string) ([]swarmServiceJsonResponse, error) {
	var services []swarmServiceJsonResponse
	if err := fetchFromSock(socketPath, "http://docker/services", &services); err != nil {
		return nil, err
	}

	return services, nil
}

func fetchAllSwarmTasksFromSock(socketPath string) ([]swarmTaskJsonResponse, error) {
	var tasks []swarmTaskJsonResponse
	if err := fetchFromSock(socketPath, "http://docker/tasks", &tasks); err != nil {
		return nil, err
	}

	return tasks, nil
}
