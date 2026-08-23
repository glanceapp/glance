package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Proxmox struct {
	URL      string
	Username string
	TokenID  string
	Secret   string
	Timeout  time.Duration
}

func New(URL string, userName string, tokenID string, secret string) *Proxmox {
	return &Proxmox{
		URL:      URL,
		Username: userName,
		TokenID:  tokenID,
		Secret:   secret,
		Timeout:  15 * time.Second,
	}
}

func (p *Proxmox) setAuthorizationHeader(req *http.Request) {
	req.Header.Set("Authorization", "PVEAPIToken="+p.Username+"@pam!"+p.TokenID+"="+p.Secret)
}

func (p *Proxmox) GetClusterResources(ctx context.Context) ([]ClusterResource, error) {
	client := &http.Client{
		Timeout: p.Timeout,
	}

	request, _ := http.NewRequestWithContext(ctx, "GET", p.URL+"/api2/json/cluster/resources", nil)
	p.setAuthorizationHeader(request)

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("sending request to cluster resources: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 cluster resources response status: %s", response.Status)
	}

	var result multipleResponse[ClusterResource]
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding cluster resources response: %w", err)
	}

	return result.Data, nil
}

func (p *Proxmox) GetNodeStatus(ctx context.Context, node string) (*NodeStatus, error) {
	client := &http.Client{
		Timeout: p.Timeout,
	}

	request, _ := http.NewRequestWithContext(ctx, "GET", p.URL+"/api2/json/nodes/"+node+"/status", nil)
	p.setAuthorizationHeader(request)

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("sending request to node status: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 node status response status: %s", response.Status)
	}

	var result singleResponse[NodeStatus]
	if err = json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding node status response: %w", err)
	}

	return &result.Data, nil
}
