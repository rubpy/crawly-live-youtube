package youtube

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rubpy/crawly-live-youtube/xmlapi"
)

//////////////////////////////////////////////////

func (cr *Crawler) ChannelID(channelURL string) (channelID string, ok bool) {
	return cr.loadChannelID(channelURL)
}

func (cr *Crawler) loadChannelID(channelURL string) (channelID string, ok bool) {
	return cr.channelIDCache.Load(channelURL)
}

func (cr *Crawler) storeChannelID(channelURL string, channelID string) {
	cr.channelIDCache.Store(channelURL, channelID)
}

func (cr *Crawler) canonicalHandle(handle Handle) Handle {
	if handle.Type == HandleChannelURL {
		if channelID, ok := cr.loadChannelID(handle.Value); ok {
			handle = ChannelID(channelID)
		}
	}

	return handle
}

func (cr *Crawler) IsTracked(handle Handle) bool {
	return cr.Crawler.IsTracked(cr.canonicalHandle(handle))
}

//////////////////////////////////////////////////

func (cr *Crawler) CheckLiveVideoState(ctx context.Context, videoID string) (live bool, finished bool, err error) {
	if videoID == "" || !IsValidVideoID(videoID) {
		err = InvalidVideoID
		return
	}

	if cr.service == nil {
		err = NilService
		return
	}

	if ctx == nil {
		ctx = context.Background()
	} else {
		if err = ctx.Err(); err != nil {
			return
		}
	}

	part := []string{"liveStreamingDetails"}
	call := cr.service.Videos.List(part)
	call.Context(ctx)
	call.Id(videoID)

	resp, err := call.Do()
	if err != nil {
		err = fmt.Errorf("youtube.VideosService.List: %w", err)
		return
	}

	for _, item := range resp.Items {
		if item.Id != videoID || item.LiveStreamingDetails == nil {
			continue
		}
		d := item.LiveStreamingDetails

		if d.ActualEndTime != "" {
			finished = true
		} else {
			if d.ActualStartTime != "" {
				live = true
			}
		}

		return
	}

	return
}

func (cr *Crawler) FetchChannelXMLFeed(ctx context.Context, channelID string) (feed *xmlapi.ChannelFeed, err error) {
	if channelID == "" || !IsValidChannelID(channelID) {
		err = InvalidChannelID
		return
	}

	if cr.client == nil {
		err = NilClient
		return
	}

	if ctx == nil {
		ctx = context.Background()
	} else {
		if err = ctx.Err(); err != nil {
			return
		}
	}

	feedURL := &url.URL{
		Scheme: "https",
		Host:   "www.youtube.com",
		Path:   "feeds/videos.xml",
	}
	q := feedURL.Query()
	q.Set("channel_id", channelID)
	q.Set(nonceKey, generateNonce())
	feedURL.RawQuery = q.Encode()

	rawFeedURL := feedURL.String()

	resp, err := cr.client.Request(ctx, "GET", rawFeedURL, nil, http.Header{
		"Cookie": {generateConsentCookie()},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	feed, err = xmlapi.ParseChannelFeed(body)
	if err != nil {
		return nil, err
	}

	return feed, nil
}

func (cr *Crawler) FetchChannelIndex(ctx context.Context, channelURL string) (index *xmlapi.ChannelIndex, err error) {
	if channelURL == "" || !IsValidChannelURL(channelURL) {
		err = InvalidChannelURL
		return
	}

	if cr.client == nil {
		err = NilClient
		return
	}

	if ctx == nil {
		ctx = context.Background()
	} else {
		if err = ctx.Err(); err != nil {
			return
		}
	}

	resp, err := cr.client.Request(ctx, "GET", channelURL, nil, http.Header{
		"Cookie": {generateConsentCookie()},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	index, err = xmlapi.ParseChannelIndex(body)
	if err != nil {
		return nil, err
	}

	return index, nil
}

// NOTE: thumbnailURL is an optional 'hint'.
func (cr *Crawler) CheckLiveVideoThumbnail(ctx context.Context, videoID string, thumbnailURL string) (exists bool, err error) {
	if videoID == "" || !IsValidVideoID(videoID) {
		err = InvalidVideoID
		return
	}

	if cr.client == nil {
		err = NilClient
		return
	}

	if ctx == nil {
		ctx = context.Background()
	} else {
		if err = ctx.Err(); err != nil {
			return
		}
	}

	var u *url.URL
	var uerr error

	if thumbnailURL != "" {
		u, uerr = url.Parse(thumbnailURL)
		if uerr == nil {
			containsExtension := false

			for _, suffix := range validVideoThumbnailSuffixes {
				if strings.Contains(u.Path, suffix) {
					containsExtension = true
					break
				}
			}

			if !containsExtension {
				u = nil
			}
		}
	}

	if u == nil || uerr != nil {
		thumbnailURL = "https://i.ytimg.com/vi/" + url.PathEscape(videoID) + "/sddefault.jpg"
		u, uerr = url.Parse(thumbnailURL)
	}

	if u == nil || uerr != nil {
		err = InvalidVideoThumbnailURL
		return
	}

	sepPos := strings.LastIndex(u.Path, ".")
	if sepPos < 0 {
		// NOTE: should never happen; weird edge case.

		err = InvalidVideoThumbnailURL
		return
	}

	pathFile := u.Path[:sepPos]
	if !strings.Contains(pathFile, liveVideoThumbnailFilenameSuffix) {
		var newPath strings.Builder

		newPath.WriteString(pathFile)
		newPath.WriteString(liveVideoThumbnailFilenameSuffix)
		newPath.WriteString(u.Path[sepPos:])

		u.Path = newPath.String()
	}

	q := u.Query()
	q.Set(nonceKey, generateNonce())
	u.RawQuery = q.Encode()

	thumbnailURL = u.String()

	resp, err := cr.client.Request(ctx, "GET", thumbnailURL, nil, http.Header{
		"Cookie": {generateConsentCookie()},
	})
	if err != nil {
		return
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 404:
		return false, nil

	case 200, 302, 304:
		return true, nil
	}

	return false, UncertainLiveVideoThumbnail
}

//////////////////////////////////////////////////

type VideoCandidate struct {
	ID          string    `json:"id"`
	ChannelID   string    `json:"channel_id"`
	LastProcess time.Time `json:"last_process"`

	Live             bool      `json:"live"`
	LiveGenuine      bool      `json:"live_genuine"`
	LiveCheckAttempt int       `json:"live_check_attempt"`
	LastLive         time.Time `json:"last_live"`

	LivestreamFinished     bool      `json:"livestream_finished"`
	LastLivestreamFinished time.Time `json:"last_livestream_finished"`

	NotLivestream     bool      `json:"not_livestream"`
	LastNotLivestream time.Time `json:"last_not_livestream"`
}

var (
	ExceededCheckVideoTimeout = errors.New("exceeded check video timeout")
)

//////////////////////////////////////////////////

var (
	InvalidChannelID            = errors.New("invalid channel ID")
	InvalidChannelURL           = errors.New("invalid channel URL")
	InvalidVideoID              = errors.New("invalid video ID")
	InvalidVideoThumbnailURL    = errors.New("invalid video thumbnail URL")
	UncertainLiveVideoThumbnail = errors.New("uncertain live video thumbnail status")
)

func IsValidChannelID(s string) bool  { return xmlapi.IsValidChannelID(s) }
func IsValidChannelURL(s string) bool { return isValidURL(s) }
func IsValidVideoID(s string) bool    { return xmlapi.IsValidVideoID(s) }

var (
	validVideoThumbnailSuffixes      = []string{".jpg", ".webp"}
	liveVideoThumbnailFilenameSuffix = "_live"
)

func generateConsentCookie() string {
	return "SOCS=CAESEwgDEgk0ODE3Nzk3MjQaAmVuIAEaBgiA_LyaBg"
}
