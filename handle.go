package youtube

import (
	"strconv"
	"strings"

	"github.com/rubpy/crawly"
)

//////////////////////////////////////////////////

type Handle struct {
	Type  HandleType
	Value string
}

func (h Handle) Valid() bool {
	return h.Type != 0 && h.Value != ""
}

func (h Handle) Equal(handle crawly.Handle) bool {
	if hh, ok := handle.(Handle); ok {
		return hh.Type == h.Type && hh.Value == h.Value
	}

	return false
}

func (h Handle) String() string {
	var s strings.Builder
	s.WriteRune('{')
	s.WriteString(h.Type.String())
	s.WriteString(":")
	s.WriteString(strconv.Quote(h.Value))
	s.WriteRune('}')

	return s.String()
}

type HandleType uint

const (
	HandleChannelID HandleType = (iota + 1)
	HandleChannelURL
)

func (ht HandleType) String() string {
	switch ht {
	case HandleChannelID:
		return "ChannelID"
	case HandleChannelURL:
		return "ChannelURL"
	}

	return ""
}

//////////////////////////////////////////////////

func ChannelID(channelID string) Handle {
	return Handle{HandleChannelID, channelID}
}

func ChannelURL(channelURL string) Handle {
	return Handle{HandleChannelURL, channelURL}
}
