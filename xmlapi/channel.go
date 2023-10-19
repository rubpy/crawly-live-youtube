package xmlapi

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

//////////////////////////////////////////////////

type ChannelIndex struct {
	ChannelID string
}

func (res ChannelIndex) String() string {
	var s strings.Builder

	s.WriteString("{ChannelIndex:[channelID:")
	s.WriteString(strconv.Quote(res.ChannelID))
	s.WriteString("]}")

	return s.String()
}

func ParseChannelIndex(b []byte) (*ChannelIndex, error) {
	if len(b) == 0 {
		return nil, errors.New("b is empty")
	}

	p := b[:]
	linkTagStart := []byte("<link")
	linkTagEnd := []byte(">")

	linkTags := []byte("<html><body>")

	for {
		startPos := bytes.Index(p, linkTagStart)
		if startPos < 0 {
			break
		}
		p = p[startPos:]

		endPos := bytes.Index(p, linkTagEnd)
		if endPos < 0 {
			break
		}
		linkTag := p[:endPos+1]
		p = p[endPos+1:]

		if len(linkTag) > len(linkTagStart)+len(linkTagEnd) {
			linkTags = append(linkTags, linkTag...)
		}
	}

	linkTags = append(linkTags, []byte("</body></html>")...)

	extract := func(b []byte) (channelID string, err error) {
		root, err := html.Parse(bytes.NewReader(b))
		if err != nil || root == nil {
			return
		}

		skipIntoChild := func(node *html.Node, nodeType html.NodeType, atomType atom.Atom) *html.Node {
			for {
				if node == nil {
					return nil
				}

				if (nodeType == 0 || node.Type == nodeType) && (atomType == 0 || node.DataAtom == atomType) {
					node = node.FirstChild
					break
				}

				node = node.NextSibling
			}

			return node
		}

		node := skipIntoChild(root, html.DocumentNode, 0)
		if node == nil {
			return
		}

		node = skipIntoChild(node, html.ElementNode, atom.Html)
		if node == nil {
			return
		}

		node = skipIntoChild(node, html.ElementNode, atom.Body)
		if node == nil {
			return
		}

		for node != nil {
			var attrs struct {
				rel      string
				itemprop string
				typ      string
				href     string
			}

			if node.Type != html.ElementNode || node.DataAtom != atom.Link || len(node.Attr) == 0 {
				goto nextNode
			}

			for _, attr := range node.Attr {
				k := strings.ToLower(attr.Key)

				switch k {
				case "rel":
					attrs.rel = strings.ToLower(attr.Val)
				case "itemprop":
					attrs.itemprop = strings.ToLower(attr.Val)
				case "type":
					attrs.typ = strings.ToLower(attr.Val)
				case "href":
					attrs.href = strings.TrimSpace(attr.Val)
				}
			}

			if attrs.rel == "canonical" || attrs.itemprop == "url" {
				marker := "channel/"
				markerPos := strings.Index(attrs.href, marker)
				if markerPos > 0 {
					s := attrs.href[markerPos+len(marker):]

					markerPos = strings.Index(s, "/")
					if markerPos > 0 {
						s = s[:markerPos]
					}

					if IsValidChannelID(s) {
						return s, nil
					}
				}
			} else if attrs.rel == "alternate" && strings.Contains(attrs.typ, "rss") {
				marker := "channel_id="
				markerPos := strings.Index(attrs.href, marker)
				if markerPos > 0 {
					s := attrs.href[markerPos+len(marker):]

					markerPos = strings.Index(s, "&")
					if markerPos > 0 {
						s = s[:markerPos]
					}

					if IsValidChannelID(s) {
						return s, nil
					}
				}
			}

		nextNode:
			node = node.NextSibling
		}

		return
	}

	channelID, err := extract(linkTags)
	if err != nil {
		return nil, fmt.Errorf("failed HTML extraction: %w", err)
	}
	if channelID == "" {
		return nil, errors.New("failed HTML extraction")
	}

	return &ChannelIndex{
		ChannelID: channelID,
	}, nil
}

//////////////////////////////////////////////////

type ChannelFeedEntry struct {
	ID string `xml:"id"`

	VideoID   string `xml:"videoId"`
	ChannelID string `xml:"channelId"`

	Title     string  `xml:"title"`
	Links     []Link  `xml:"link,omitempty"`
	Author    *Author `xml:"author,omitempty"`
	Published string  `xml:"published"`
	Updated   string  `xml:"updated"`

	MediaGroups []MediaGroup `xml:"group,omitempty"`
}

type ChannelFeed struct {
	XMLName xml.Name `xml:"feed"`

	Links []Link `xml:"link,omitempty"`

	Title     string  `xml:"title"`
	Author    *Author `xml:"author,omitempty"`
	Published string  `xml:"published"`

	Entries []ChannelFeedEntry `xml:"entry,omitempty"`
}

type ChannelFeedVideo struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`

	Title        string `json:"title"`
	Description  string `json:"description"`
	URL          string `json:"url"`
	ThumbnailURL string `json:"thumbnail_url"`

	Published int64 `json:"published"` // (unix timestamp; in milliseconds)
	Updated   int64 `json:"updated"`   // (unix timestamp; in milliseconds)
}

func (vid ChannelFeedVideo) String() string {
	var s strings.Builder

	s.WriteString("{ChannelFeedVideo:[id:")
	s.WriteString(strconv.Quote(vid.ID))
	s.WriteString(", channelID:")
	s.WriteString(strconv.Quote(vid.ChannelID))
	s.WriteString("]}")

	return s.String()
}

func (f *ChannelFeed) Videos() (videos []ChannelFeedVideo) {
	videos = make([]ChannelFeedVideo, 0)

	if f == nil || len(f.Entries) == 0 {
		return
	}

	ids := make([]string, 0)
	for _, entry := range f.Entries {
		if len(entry.VideoID) == 0 {
			continue
		}

		videoID := entry.VideoID
		videoIDUnique := true
		for _, id := range ids {
			if id == videoID {
				videoIDUnique = false
				break
			}
		}
		if !videoIDUnique {
			continue
		}

		published, _ := parseDate(entry.Published)
		updated, _ := parseDate(entry.Updated)

		vid := ChannelFeedVideo{
			ID:        videoID,
			ChannelID: entry.ChannelID,

			Title:        entry.Title,
			Description:  "",
			URL:          "https://www.youtube.com/watch?v=" + videoID,
			ThumbnailURL: "",

			Published: published,
			Updated:   updated,
		}

		if len(entry.Links) > 0 {
			for _, link := range entry.Links {
				if len(link.Href) == 0 || strings.ToLower(link.Rel) != "alternate" {
					continue
				}

				if isValidURL(link.Href) {
					continue
				}

				vid.URL = link.Href
				break
			}
		}

		if len(entry.MediaGroups) > 0 {
			for _, mg := range entry.MediaGroups {
				vid.Description = mg.Description

				if mg.Thumbnail != nil && len(mg.Thumbnail.URL) > 0 {
					if isValidURL(mg.Thumbnail.URL) {
						vid.ThumbnailURL = mg.Thumbnail.URL
						break
					}
				}
			}
		}

		videos = append(videos, vid)
		ids = append(ids, vid.ID)
	}

	return
}

func ParseChannelFeed(b []byte) (*ChannelFeed, error) {
	if len(b) == 0 {
		return nil, errors.New("b is empty")
	}

	feed := &ChannelFeed{}
	if err := xml.Unmarshal(b, feed); err != nil {
		return nil, err
	}

	return feed, nil
}

// Checks (roughly) if the given string is a valid YouTube channel ID.
func IsValidChannelID(s string) bool {
	n := len(s)
	if n < 6 || n > 64 {
		return false
	}

	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
			if r == '-' || r == '_' {
				continue
			}

			return false
		}
	}

	return true
}
