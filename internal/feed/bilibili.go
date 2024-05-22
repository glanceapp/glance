package feed

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type bilibiliSpaceResponseJson struct {
	Data struct {
		Item []struct {
			Title  string `json:"title"`
			Cover  string `json:"cover"`
			Ctime  int64  `json:"ctime"`
			Author string `json:"author"`
			Bvid   string `json:"bvid"`
		} `json:"item"`
	} `json:"data"`
}

func FetchBilibiliUploads(uidList []int) (Videos, error) {
	requests := make([]*http.Request, 0, len(uidList))
	u := "https://app.bilibili.com/x/v2/space/archive/cursor?vmid="
	for i := range uidList {
		request, _ := http.NewRequest("GET", u+strconv.Itoa(uidList[i]), nil)
		request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
		request.Header.Set("Referer", "https://www.bilibili.com/")

		requests = append(requests, request)
	}

	job := newJob(decodeJsonFromRequestTask[bilibiliSpaceResponseJson](defaultClient), requests).withWorkers(30)

	responses, errs, err := workerPoolDo(job)

	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoContent, err)
	}

	videos := make(Videos, 0, len(uidList)*15)
	var failed int
	for i := range responses {
		if errs[i] != nil {
			failed++
			slog.Error("Failed to fetch bilibili feed", "uid", uidList[i], "error", errs[i])
			continue
		}
		response := responses[i]
		for j := range response.Data.Item {
			video := &response.Data.Item[j]
			videoUrl := `https://www.bilibili.com/video/` + video.Bvid
			videos = append(videos, Video{
				ThumbnailUrl: video.Cover,
				Title:        video.Title,
				Url:          strings.ReplaceAll(videoUrl, "http://", "https://"),
				Author:       video.Author,
				AuthorUrl:    `https://space.bilibili.com/` + strconv.Itoa(uidList[i]),
				TimePosted:   time.Unix(video.Ctime, 0),
			})
		}
	}
	if len(videos) == 0 {
		return nil, ErrNoContent
	}

	videos.SortByNewest()

	if failed > 0 {
		return videos, fmt.Errorf("%w: missing videos from %d up", ErrPartialContent, failed)
	}

	return videos, nil
}
