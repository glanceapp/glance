package glance

import (
	"encoding/json"
	"fmt"
	"testing"
)

// TestFetchPodcastChannel test fetch single channel
func TestFetchPodcastChannel(t *testing.T) {
	channel := &podcastChannel{
		podcastID: "1482731836",
		region:    "cn",
	}
	resp, err := fetchPodcastChannel(channel)
	if err != nil {
		fmt.Printf("\nerr:%+v", err)
	}
	r, _ := json.Marshal(resp)
	fmt.Printf("\nresp: \n%+v", string(r))
}

// TestFetchPodcastChannels test fetch single channel
func TestFetchPodcastChannels(t *testing.T) {
	channel := &podcastChannel{
		podcastID: "1482731836",
		region:    "cn",
	}
	resp, err := fetchPodcastChannels([]*podcastChannel{channel})
	if err != nil {
		fmt.Printf("\nerr:%+v", err)
	}
	r, _ := json.Marshal(resp)
	fmt.Printf("\nresp: \n%+v", string(r))
}
