package sources

import (
	"crypto/tls"
	"fmt"
	"gopkg.in/yaml.v3"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

type proxyOptionsField struct {
	URL           string        `yaml:"url"`
	AllowInsecure bool          `yaml:"allow-insecure"`
	Timeout       durationField `yaml:"timeout"`
	client        *http.Client  `yaml:"-"`
}

func (p *proxyOptionsField) UnmarshalYAML(node *yaml.Node) error {
	type proxyOptionsFieldAlias proxyOptionsField
	alias := (*proxyOptionsFieldAlias)(p)
	var proxyURL string

	if err := node.Decode(&proxyURL); err != nil {
		if err := node.Decode(alias); err != nil {
			return err
		}
	}

	if proxyURL == "" && p.URL == "" {
		return nil
	}

	if p.URL != "" {
		proxyURL = p.URL
	}

	parsedUrl, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("parsing proxy URL: %v", err)
	}

	var timeout = defaultClientTimeout
	if p.Timeout > 0 {
		timeout = time.Duration(p.Timeout)
	}

	p.client = &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(parsedUrl),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: p.AllowInsecure},
		},
	}

	return nil
}

var durationFieldPattern = regexp.MustCompile(`^(\d+)(s|m|h|d)$`)

type durationField time.Duration

func (d *durationField) UnmarshalYAML(node *yaml.Node) error {
	var value string

	if err := node.Decode(&value); err != nil {
		return err
	}

	matches := durationFieldPattern.FindStringSubmatch(value)

	if len(matches) != 3 {
		return fmt.Errorf("invalid duration format: %s", value)
	}

	duration, err := strconv.Atoi(matches[1])
	if err != nil {
		return err
	}

	switch matches[2] {
	case "s":
		*d = durationField(time.Duration(duration) * time.Second)
	case "m":
		*d = durationField(time.Duration(duration) * time.Minute)
	case "h":
		*d = durationField(time.Duration(duration) * time.Hour)
	case "d":
		*d = durationField(time.Duration(duration) * 24 * time.Hour)
	}

	return nil
}
