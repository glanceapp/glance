package feed

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFetchBilibiliUploads(t *testing.T) {
	videos, err := FetchBilibiliUploads([]int{50329118})
	assert.Nil(t, err)
	assert.True(t, len(videos) > 0)
}
