package glance

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"
)

const httpTestRequestTimeout = 15 * time.Second

var diagnosticSteps = []diagnosticStep{
	{
		name: "resolve cloudflare.com through Cloudflare DoH",
		fn: func() (string, error) {
			return testHttpRequestWithHeaders("GET", "https://1.1.1.1/dns-query?name=cloudflare.com", map[string]string{
				"accept": "application/dns-json",
			}, 200)
		},
	},
	{
		name: "resolve cloudflare.com through Google DoH",
		fn: func() (string, error) {
			return testHttpRequest("GET", "https://8.8.8.8/resolve?name=cloudflare.com", 200)
		},
	},
	{
		name: "resolve github.com",
		fn: func() (string, error) {
			return testDNSResolution("github.com")
		},
	},
	{
		name: "resolve reddit.com",
		fn: func() (string, error) {
			return testDNSResolution("reddit.com")
		},
	},
	{
		name: "resolve twitch.tv",
		fn: func() (string, error) {
			return testDNSResolution("twitch.tv")
		},
	},
	{
		name: "fetch data from YouTube RSS feed",
		fn: func() (string, error) {
			return testHttpRequest("GET", "https://www.youtube.com/feeds/videos.xml?channel_id=UCZU9T1ceaOgwfLRq7OKFU4Q", 200)
		},
	},
	{
		name: "fetch data from Twitch.tv GQL",
		fn: func() (string, error) {
			// this should always return 0 bytes, we're mainly looking for a 200 status code
			return testHttpRequest("OPTIONS", "https://gql.twitch.tv/gql", 200)
		},
	},
	{
		name: "fetch data from GitHub API",
		fn: func() (string, error) {
			return testHttpRequest("GET", "https://api.github.com", 200)
		},
	},
	{
		name: "fetch data from Open-Meteo API",
		fn: func() (string, error) {
			return testHttpRequest("GET", "https://geocoding-api.open-meteo.com/v1/search?name=London", 200)
		},
	},
	{
		name: "fetch data from Reddit API",
		fn: func() (string, error) {
			return testHttpRequestWithHeaders("GET", "https://www.reddit.com/search.json", map[string]string{
				"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:137.0) Gecko/20100101 Firefox/137.0",
			}, 200)
		},
	},
	{
		name: "fetch data from Yahoo finance API",
		fn: func() (string, error) {
			return testHttpRequestWithHeaders("GET", "https://query1.finance.yahoo.com/v8/finance/chart/NVDA", map[string]string{
				"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:137.0) Gecko/20100101 Firefox/137.0",
			}, 200)
		},
	},
	{
		name: "fetch data from Hacker News Firebase API",
		fn: func() (string, error) {
			return testHttpRequest("GET", "https://hacker-news.firebaseio.com/v0/topstories.json", 200)
		},
	},
	{
		name: "fetch data from Docker Hub API",
		fn: func() (string, error) {
			return testHttpRequest("GET", "https://hub.docker.com/v2/namespaces/library/repositories/ubuntu/tags/latest", 200)
		},
	},
}

func runDiagnostic() {
	fmt.Println("```")
	fmt.Println("Glance version: " + buildVersion)
	fmt.Println("Go version: " + runtime.Version())
	fmt.Printf("Platform: %s / %s / %d CPUs\n", runtime.GOOS, runtime.GOARCH, runtime.NumCPU())
	fmt.Println("In Docker container: " + ternary(isRunningInsideDockerContainer(), "yes", "no"))

	fmt.Printf("\nChecking network connectivity, this may take up to %d seconds...\n\n", int(httpTestRequestTimeout.Seconds()))

	var wg sync.WaitGroup
	for i := range diagnosticSteps {
		step := &diagnosticSteps[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			step.extraInfo, step.err = step.fn()
			step.elapsed = time.Since(start)
		}()
	}
	wg.Wait()

	for _, step := range diagnosticSteps {
		var extraInfo string

		if step.extraInfo != "" {
			extraInfo = "| " + step.extraInfo + " "
		}

		fmt.Printf(
			"%s %s %s| %dms\n",
			ternary(step.err == nil, "✓ Can", "✗ Can't"),
			step.name,
			extraInfo,
			step.elapsed.Milliseconds(),
		)

		if step.err != nil {
			fmt.Printf("└╴ error: %v\n", step.err)
		}
	}
	fmt.Println("```")
}

type diagnosticStep struct {
	name      string
	fn        func() (string, error)
	extraInfo string
	err       error
	elapsed   time.Duration
}

func testHttpRequest(method, url string, expectedStatusCode int) (string, error) {
	return testHttpRequestWithHeaders(method, url, nil, expectedStatusCode)
}

func testHttpRequestWithHeaders(method, url string, headers map[string]string, expectedStatusCode int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), httpTestRequestTimeout)
	defer cancel()

	request, _ := http.NewRequestWithContext(ctx, method, url, nil)
	for key, value := range headers {
		request.Header.Add(key, value)
	}

	response, err := defaultHTTPClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	printableBody := strings.ReplaceAll(string(body), "\n", "")
	if len(printableBody) > 50 {
		printableBody = printableBody[:50] + "..."
	}
	if len(printableBody) > 0 {
		printableBody = ", " + printableBody
	}

	extraInfo := fmt.Sprintf("%d bytes%s", len(body), printableBody)

	if response.StatusCode != expectedStatusCode {
		return extraInfo, fmt.Errorf("expected status code %d, got %d", expectedStatusCode, response.StatusCode)
	}

	return extraInfo, nil
}

func testDNSResolution(domain string) (string, error) {
	ips, err := net.LookupIP(domain)

	var ipStrings []string
	if err == nil {
		for i := range ips {
			ipStrings = append(ipStrings, ips[i].String())
		}
	}

	return strings.Join(ipStrings, ", "), err
}
