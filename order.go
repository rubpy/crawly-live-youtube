package youtube

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/rubpy/crawly"
	"github.com/rubpy/crawly/clog"
)

//////////////////////////////////////////////////

type OrderData struct{}

func (cr *Crawler) orderHandler(ctx context.Context, order *crawly.Order, result *crawly.TrackingResult) error {
	handle, ok := order.Handle.(Handle)
	if !ok || !handle.Valid() {
		return crawly.InvalidHandle
	}

	data, _ := order.Data.(OrderData)
	defer func() {
		order.Data = data
	}()

	if handle.Type == HandleChannelURL {
		if !IsValidChannelURL(handle.Value) {
			return crawly.InvalidHandle
		}

		channelID, ok := cr.loadChannelID(handle.Value)
		if !ok {
			lp := clog.Params{
				Message: "fetchChannelIndex",
				Level:   slog.LevelDebug,

				Values: clog.ParamGroup{
					"channelURL": handle.Value,
				},
			}

			index, err := cr.FetchChannelIndex(ctx, handle.Value)
			if err == nil {
				if IsValidChannelID(index.ChannelID) {
					cr.storeChannelID(handle.Value, index.ChannelID)

					channelID = index.ChannelID
				}
			} else {
				err = fmt.Errorf("FetchChannelIndex: %w", err)
			}

			lp.Err = err
			cr.Log(ctx, lp)

			if err != nil {
				return err
			}
		}

		if channelID == "" {
			return InvalidChannelID
		}
		handle = Handle{
			Type:  HandleChannelID,
			Value: channelID,
		}
		result.Entity.Value.Handle = handle
	}

	switch handle.Type {
	case HandleChannelID:
		if !IsValidChannelID(handle.Value) {
			return crawly.InvalidHandle
		}

	default:
		return crawly.InvalidHandle
	}

	return nil
}
