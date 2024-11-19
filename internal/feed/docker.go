package feed

import (
	"context"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"strings"
)

const (
	dockerAPIVersion    = "1.24"
	dockerGlanceEnable  = "glance.enable"
	dockerGlanceTitle   = "glance.title"
	dockerGlanceUrl     = "glance.url"
	dockerGlanceIconUrl = "glance.iconUrl"
)

type DockerContainer struct {
	Id      string
	Image   string
	Title   string
	URL     string
	IconURL string
	Status  string
	State   string
}

func FetchDockerContainers(ctx context.Context) ([]DockerContainer, error) {
	apiClient, err := client.NewClientWithOpts(client.WithVersion(dockerAPIVersion), client.FromEnv)
	if err != nil {
		return nil, err
	}
	defer apiClient.Close()

	containers, err := apiClient.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, err
	}

	var results []DockerContainer

	for _, c := range containers {
		isGlanceEnabled := getLabelValue(c.Labels, dockerGlanceEnable, "true")

		if isGlanceEnabled != "true" {
			continue
		}

		results = append(results, DockerContainer{
			Id:      c.ID,
			Image:   c.Image,
			Title:   getLabelValue(c.Labels, dockerGlanceTitle, strings.Join(c.Names, "")),
			URL:     getLabelValue(c.Labels, dockerGlanceUrl, ""),
			IconURL: getLabelValue(c.Labels, dockerGlanceIconUrl, "si:docker"),
			Status:  c.Status,
			State:   c.State,
		})
	}

	return results, nil
}

// getLabelValue get string value associated to a label.
func getLabelValue(labels map[string]string, labelName, defaultValue string) string {
	if value, ok := labels[labelName]; ok && len(value) > 0 {
		return value
	}
	return defaultValue
}
