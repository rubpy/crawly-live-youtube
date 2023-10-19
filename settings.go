package youtube

import (
	"time"

	"github.com/rubpy/crawly"
)

//////////////////////////////////////////////////

type CrawlerSettings struct {
	crawly.CrawlerSettings

	// Stop looking for more live videos after this number is reached (e.g.,
	// if this is set to 1, crawler will not scan remaining videos once it has
	// found one livestream).
	StopAfterLiveVideos int

	MinimumFetchChannelFeedDelay       time.Duration
	MaximumCachedNotLivestreamAge      time.Duration
	MaximumCachedLivestreamFinishedAge time.Duration
	MinimumCheckVideoDelay             time.Duration
	MaximumVideoAge                    time.Duration

	CheckVideoTimeout time.Duration
}

var DefaultSettings = CrawlerSettings{
	CrawlerSettings: crawly.DefaultCrawlerSettings,

	StopAfterLiveVideos: 1,

	MinimumFetchChannelFeedDelay:       45 * time.Second,
	MaximumCachedNotLivestreamAge:      15 * time.Minute,
	MaximumCachedLivestreamFinishedAge: 60 * time.Minute,
	MinimumCheckVideoDelay:             30 * time.Second,
	MaximumVideoAge:                    60 * 24 * time.Hour,

	CheckVideoTimeout: 10 * time.Second,
}

//////////////////////////////////////////////////

func (cr *Crawler) loadSettings() CrawlerSettings {
	return cr.settings.Load()
}

func (cr *Crawler) setSettings(settings CrawlerSettings) {
	cr.settings.Store(settings)
	crawly.SetCrawlerSettings(&cr.Crawler, settings.CrawlerSettings)
}

func (cr *Crawler) Settings() CrawlerSettings {
	return cr.loadSettings()
}

func (cr *Crawler) SetSettings(settings CrawlerSettings) {
	cr.setSettings(settings)
}
