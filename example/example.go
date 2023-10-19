package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/lmittmann/tint"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"

	"github.com/rubpy/crawly"
	cyoutube "github.com/rubpy/crawly-live-youtube"
	"github.com/rubpy/crawly/clog"
)

//////////////////////////////////////////////////

var testHandles = []cyoutube.Handle{
	cyoutube.ChannelURL("https://www.youtube.com/@LofiGirl"),
}

var (
	/* NOTE: fill with your own YouTube Data API v3 key. */
	youtubeAPIKey = ""
)

const logHeader = "[example] "

var sessionSettings = crawly.SessionSettings{
	Interval:          30 * time.Second,
	SinglePassTimeout: 45 * time.Second,

	Paused:    false,
	PauseIdle: false,
}

var crawlerSettings = cyoutube.DefaultSettings

//////////////////////////////////////////////////

func main() {
	ctx := context.Background()

	logFile := os.Stdout
	logger := slog.New(
		tint.NewHandler(logFile, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.DateTime,
		}),
	)

	if youtubeAPIKey == "" {
		panic(errors.New("YouTube API key is empty"))
	}
	srv, err := youtube.NewService(ctx, option.WithAPIKey(youtubeAPIKey))
	if err != nil {
		panic(fmt.Errorf("youtube.NewService: %w", err))
	}

	cr, err := cyoutube.NewCrawler(
		cyoutube.WithLogger(logger),
		cyoutube.WithService(srv),
		cyoutube.WithSettings(crawlerSettings),
	)
	if err != nil {
		panic(fmt.Errorf("cyoutube.NewCrawler: %w", err))
	}

	for _, handle := range testHandles {
		if _, err := cr.Track(ctx, handle); err != nil {
			panic(fmt.Errorf("cyoutube.Track: %w", err))
		}
	}

	go func(ctx context.Context, cr *cyoutube.Crawler) {
		l := cr.Listen()
		defer l.Discard()

		ch := l.Channel()
		for {
			select {
			case <-ctx.Done():
				return

			case result, ok := <-ch:
				if !ok {
					// Result channel has been closed.

					return
				}

				orders := map[crawly.Handle]string{}
				entitiesLiveVideos := map[crawly.Handle][]string{}
				for _, tr := range result.Orders {
					order := tr.Order.Value

					orders[order.Handle] = order.Command.String()
				}
				for _, tr := range result.Entities {
					entity := tr.Entity.Value

					data, ok := entity.Data.(cyoutube.EntityData)
					if !ok {
						continue
					}

					entitiesLiveVideos[entity.Handle] = data.LiveVideos
				}

				cr.Log(ctx, clog.Params{
					Message: fmt.Sprintf(logHeader+"%T: result", cr),
					Level:   slog.LevelInfo,

					Values: clog.ParamGroup{
						"sessionID": result.SessionID,

						"orders":   orders,
						"entities": entitiesLiveVideos,
					},
				})
			}
		}
	}(ctx, cr)

	printHelp()

	if err = cr.Start(ctx, sessionSettings); err != nil {
		panic(fmt.Errorf("cyoutube.Start: %w", err))
	}
	defer cr.Stop(ctx)

	// ------------------------------

	interruptSignal := make(chan os.Signal, 1)
	signal.Notify(interruptSignal, os.Interrupt)

	ui := NewUI()
	ui.BindKey(UIKeySubject{Key: keyboard.KeySpace}, func(_ *UI, e *UIKeyEvent) error {
		var verb string
		if cr.Paused() {
			cr.Resume(ctx)
			verb = "resumed"
		} else {
			cr.Pause(ctx)
			verb = "paused"
		}

		if verb != "" {
			cr.Log(ctx, clog.Params{
				Message: fmt.Sprintf(logHeader+"%T: %s", cr, verb),
				Level:   slog.LevelInfo,
			})
		}

		return nil
	})
	ui.BindKey(UIKeySubject{Rune: 'q'}, func(_ *UI, e *UIKeyEvent) error {
		e.StopPropagation()

		cr.Log(ctx, clog.Params{
			Message: logHeader + "quitting",
			Level:   slog.LevelInfo,
		})

		interruptSignal <- syscall.SIGINT

		return nil
	})
	ui.BindKey(UIKeySubject{Rune: 'i'}, func(_ *UI, e *UIKeyEvent) error {
		lp := clog.Params{
			Message: fmt.Sprintf(logHeader+"%T: immediate", cr),
			Level:   slog.LevelInfo,
		}

		_, lp.Err = cr.Immediate(ctx, 0)

		cr.Log(ctx, lp)
		return nil
	})
	ui.BindKey(UIKeySubject{Rune: 'u'}, func(_ *UI, e *UIKeyEvent) error {
		lp := clog.Params{
			Message: fmt.Sprintf(logHeader+"%T: untracking all", cr),
			Level:   slog.LevelInfo,
		}

		_, lp.Err = cr.UntrackAll(ctx)

		cr.Log(ctx, lp)
		return nil
	})
	ui.BindKey(UIKeySubject{Key: keyboard.KeyEnter}, func(_ *UI, e *UIKeyEvent) error {
		fmt.Println()

		return nil
	})

	// ------------------------------

	go ui.Listen(ctx)
	defer ui.Close()

	<-interruptSignal
}

func printHelp() {
	fmt.Println("========================================")
	fmt.Println(" Controls:")
	fmt.Println("   Q     --- quit")
	fmt.Println("   Space --- pause/resume")
	fmt.Println("   I     --- trigger an immediate crawl")
	fmt.Println("   U     --- untrack all handles")

	fmt.Println("========================================")
	fmt.Println(" Test handles:")
	for _, handle := range testHandles {
		fmt.Println("  ", handle.String())
	}

	fmt.Println("========================================")
	fmt.Println()
}
