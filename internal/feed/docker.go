package feed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type DockerContainer struct {
	Id     string
	Image  string
	Names  []string
	Status string
	State  string
	Labels map[string]string
}

func FetchDockerContainers(URL string) ([]DockerContainer, error) {
	hostURL, err := parseHostURL(URL)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		MaxIdleConns:    6,
		IdleConnTimeout: 30 * time.Second,
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial(hostURL.Scheme, hostURL.Host)
		},
	}

	cli := &http.Client{
		Transport:     transport,
		CheckRedirect: checkRedirect,
	}

	resp, err := cli.Get("http://docker/containers/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results []DockerContainer
	err = json.NewDecoder(resp.Body).Decode(&results)
	return results, err
}

func parseHostURL(host string) (*url.URL, error) {
	proto, addr, ok := strings.Cut(host, "://")
	if !ok || addr == "" {
		return nil, fmt.Errorf("unable to parse docker host: %s", host)
	}

	var basePath string
	if proto == "tcp" {
		parsed, err := url.Parse(host)
		if err != nil {
			return nil, err
		}
		addr = parsed.Host
		basePath = parsed.Path
	}
	return &url.URL{
		Scheme: proto,
		Host:   addr,
		Path:   basePath,
	}, nil
}

func checkRedirect(_ *http.Request, via []*http.Request) error {
	if via[0].Method == http.MethodGet {
		return http.ErrUseLastResponse
	}
	return errors.New("unexpected redirect in response")
}
