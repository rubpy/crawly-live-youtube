package xmlapi

import "encoding/xml"

//////////////////////////////////////////////////

type MediaContent struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Width  int    `xml:"width,attr"`
	Height int    `xml:"height,attr"`
}

type MediaThumbnail struct {
	URL    string `xml:"url,attr"`
	Width  int    `xml:"width,attr"`
	Height int    `xml:"height,attr"`
}

type MediaStatistics struct {
	Views int `xml:"views,attr"`
}

type MediaStarRating struct {
	Count   int     `xml:"count,attr"`
	Average float32 `xml:"average,attr"`
	Min     int     `xml:"min,attr"`
	Max     int     `xml:"max,attr"`
}

type MediaCommunity struct {
	StarRating *MediaStarRating `xml:"starRating,omitempty"`
	Statistics *MediaStatistics `xml:"statistics,omitempty"`
}

type MediaGroup struct {
	XMLName xml.Name `xml:"group"`

	Title       string          `xml:"title"`
	Content     *MediaContent   `xml:"content,omitempty"`
	Thumbnail   *MediaThumbnail `xml:"thumbnail,omitempty"`
	Description string          `xml:"description"`
	Community   *MediaCommunity `xml:"community,omitempty"`
}
