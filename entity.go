package youtube

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/rubpy/crawly"
	"github.com/rubpy/crawly-live-youtube/xmlapi"
	"github.com/rubpy/crawly/clog"
)

//////////////////////////////////////////////////

type EntityData struct {
	Live       bool     `json:"live"`
	LiveVideos []string `json:"live_videos"`

	Feed          *xmlapi.ChannelFeed `json:"feed"`
	LastFeedFetch time.Time           `json:"last_feed_fetch"`

	FeedVideoCandidates []VideoCandidate `json:"feed_video_candidates"`
}

func (cr *Crawler) entityHandler(ctx context.Context, entity *crawly.Entity, result *crawly.TrackingResult) error {
	handle, ok := entity.Handle.(Handle)
	if !ok || !handle.Valid() {
		return crawly.InvalidHandle
	}

	data, _ := entity.Data.(EntityData)
	defer func() {
		entity.Data = data
	}()

	if handle.Type != HandleChannelID {
		return crawly.InvalidHandle
	}

	settings := cr.loadSettings()
	max := func(min time.Duration, v time.Duration) time.Duration {
		if v < min {
			return min
		}

		return v
	}

	stopAfterLiveVideos := settings.StopAfterLiveVideos
	if stopAfterLiveVideos < 1 {
		stopAfterLiveVideos = 0
	}

	minimumFetchChannelFeedDelay := max(1*time.Second, settings.MinimumFetchChannelFeedDelay)
	maximumCachedNotLivestreamAge := max(1*time.Second, settings.MaximumCachedNotLivestreamAge)
	maximumCachedLivestreamFinishedAge := max(1*time.Second, settings.MaximumCachedLivestreamFinishedAge)
	minimumCheckVideoDelay := max(1*time.Second, settings.MinimumCheckVideoDelay)
	maximumVideoAge := max(0*time.Second, settings.MaximumVideoAge)
	checkVideoTimeout := max(0*time.Second, settings.CheckVideoTimeout)

	getVideoCandidates := func(feed *xmlapi.ChannelFeed, channelID string) (vcs []VideoCandidate, err error) {
		if feed == nil {
			err = errors.New("feed is nil")
			return
		}

		vids := feed.Videos()
		if len(vids) == 0 {
			return
		}

		now := time.Now()
		nowm := now.UnixMilli()
		for _, vid := range vids {
			lastTouch := vid.Updated
			if vid.Published > lastTouch {
				lastTouch = vid.Published
			}

			if lastTouch >= nowm {
				continue
			}
			if (nowm - lastTouch) > maximumVideoAge.Milliseconds() {
				continue
			}

			vcs = append(vcs, VideoCandidate{
				ID:        vid.ID,
				ChannelID: channelID,
			})
		}

		return
	}

	processVideoCandidate := func(pctx context.Context, vc *VideoCandidate) error {
		if vc == nil {
			return errors.New("vc is nil")
		}

		if pctx == nil {
			pctx = context.Background()
		} else {
			if err := pctx.Err(); err != nil {
				return err
			}
		}

		var ctx context.Context
		var cancel context.CancelFunc
		if checkVideoTimeout > 0 {
			ctx, cancel = context.WithTimeoutCause(pctx, checkVideoTimeout, ExceededCheckVideoTimeout)
		} else {
			ctx, cancel = context.WithCancel(pctx)
		}
		defer cancel()

		if time.Now().Sub(vc.LastNotLivestream) >= maximumCachedNotLivestreamAge {
			vc.NotLivestream = false

			lp := clog.Params{
				Message: "checkLiveVideoThumbnail",
				Level:   slog.LevelDebug,

				Values: clog.ParamGroup{
					"videoID":   vc.ID,
					"channelID": vc.ChannelID,
				},
			}

			thumbnailExists, err := cr.CheckLiveVideoThumbnail(ctx, vc.ID, "")
			if err == nil {
				vc.LastNotLivestream = time.Now()
				vc.NotLivestream = !thumbnailExists

				lp.Set("thumbnailExists", thumbnailExists)
			} else {
				err = fmt.Errorf("CheckLiveVideoThumbnail: %w", err)
			}

			lp.Err = err
			cr.Log(ctx, lp)

			if err != nil {
				return err
			}
		}

		if vc.NotLivestream {
			vc.Live = false
			vc.LiveCheckAttempt = 0
			vc.LivestreamFinished = false
			vc.LastLivestreamFinished = time.Time{}

			return nil
		}

		if vc.LivestreamFinished && time.Now().Sub(vc.LastLivestreamFinished) <= maximumCachedLivestreamFinishedAge {
			vc.Live = false
			vc.LiveCheckAttempt = 0

			return nil
		}

		{
			lp := clog.Params{
				Message: "checkLiveVideoState",
				Level:   slog.LevelDebug,

				Values: clog.ParamGroup{
					"videoID":   vc.ID,
					"channelID": vc.ChannelID,
				},
			}

			live, finished, err := cr.CheckLiveVideoState(ctx, vc.ID)
			vc.LastLive = time.Now()
			if err == nil {
				vc.LivestreamFinished = finished
				vc.LastLivestreamFinished = time.Now()
				vc.Live = live

				lp.Set("live", live)
				lp.Set("finished", finished)
			} else {
				err = fmt.Errorf("CheckLiveVideoState: %w", err)

				vc.LiveCheckAttempt++
			}

			lp.Err = err
			cr.Log(ctx, lp)

			if err != nil {
				return err
			}
		}

		return nil
	}

	{
		channelID := handle.Value
		data.Live = false

		if (data.Feed == nil || len(data.FeedVideoCandidates) == 0) && time.Now().Sub(data.LastFeedFetch) >= minimumFetchChannelFeedDelay {
			data.Feed = nil
			data.FeedVideoCandidates = nil
			data.LiveVideos = []string{}

			lp := clog.Params{
				Message: "fetchChannelXMLFeed",
				Level:   slog.LevelDebug,

				Values: clog.ParamGroup{
					"channelID": channelID,
				},
			}

			feed, err := cr.FetchChannelXMLFeed(ctx, channelID)
			if err == nil {
				data.Feed = feed
				data.LastFeedFetch = time.Now()
			} else {
				err = fmt.Errorf("FetchChannelXMLFeed: %w", err)
			}

			lp.Err = err
			cr.Log(ctx, lp)

			if err != nil {
				return err
			}

			data.FeedVideoCandidates, err = getVideoCandidates(feed, channelID)
			if err != nil {
				return err
			}
		}

		data.LiveVideos = []string{}
		for idx := range data.FeedVideoCandidates {
			vc := &data.FeedVideoCandidates[idx]

			if time.Now().Sub(vc.LastProcess) >= minimumCheckVideoDelay {
				err := processVideoCandidate(ctx, vc)
				vc.LastProcess = time.Now()
				if err == nil {
					vc.LiveGenuine = true
				} else {
					vc.LiveGenuine = false

					cr.Log(ctx, clog.Params{
						Message: "processVideoCandidate",
						Level:   slog.LevelError,
						Err:     err,

						Values: clog.ParamGroup{
							"videoID":   vc.ID,
							"channelID": channelID,
						},
					})
				}
			}

			if vc.LiveGenuine && vc.Live {
				data.LiveVideos = append(data.LiveVideos, vc.ID)

				if stopAfterLiveVideos > 0 && len(data.LiveVideos) >= stopAfterLiveVideos {
					break
				}
			}
		}

		data.Live = len(data.LiveVideos) > 0
	}

	return nil
}
