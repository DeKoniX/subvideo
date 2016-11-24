package main

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/mrshankly/go-twitch/twitch"
	"google.golang.org/api/googleapi/transport"
	youtube "google.golang.org/api/youtube/v3"
)

type SubVideo struct {
	TypeSub     string
	Title       string
	Channel     string
	ChannelID   string
	Game        string
	Description string
	URL         string
	ThumbURL    string
	Date        time.Time
	dateInt     int64
}

type ChannelOnline struct {
	Title     string
	Channel   string
	ChannelID string
	URL       string
	Game      string
	ThumbURL  string
}

type byDate []SubVideo

func (a byDate) Len() int           { return len(a) }
func (a byDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byDate) Less(i, j int) bool { return a[i].dateInt > a[j].dateInt }

type ClientVideo struct {
	client      *http.Client
	twClientID  string
	twUserName  string
	ytChannelID string
	TimeZone    *time.Location
}

func InitClientVideo(twClientID, ytDeveloperKey, twUserName, ytChannelID string) (clientVideo ClientVideo) {
	clientVideo.client = &http.Client{
		Transport: &transport.APIKey{Key: ytDeveloperKey},
	}
	clientVideo.twClientID = twClientID
	clientVideo.twUserName = twUserName
	clientVideo.ytChannelID = ytChannelID
	clientVideo.TimeZone, _ = time.LoadLocation("UTC")

	return clientVideo
}

func (clientVideo ClientVideo) SortVideo(n int) (subVideos []SubVideo, err error) {
	subVideos, err = clientVideo.twGetVideo(subVideos)
	if err != nil {
		return subVideos, err
	}
	subVideos, err = clientVideo.ytGetVideo(subVideos)
	if err != nil {
		return subVideos, err
	}
	sort.Sort(byDate(subVideos))

	if len(subVideos) >= n {
		return subVideos[:n], nil
	}
	return subVideos, nil
}

func (clientVideo ClientVideo) ChannelsOnline() (channelOnline []ChannelOnline, err error) {
	clientTW := twitch.NewClient(clientVideo.client)
	clientTW.ClientId = clientVideo.twClientID
	opt := &twitch.ListOptions{
		Limit:  50,
		Offset: 0,
	}

	fol, err := clientTW.Users.Follows(clientVideo.twUserName, opt)
	if err != nil {
		return channelOnline, fmt.Errorf("Twitch-Users-Follows: %s", err)
	}

	for _, follow := range fol.Follows {
		stream, err := clientTW.Streams.Channel(follow.Channel.Name)
		if err != nil {
			return channelOnline, fmt.Errorf("Twitch-Streams-Channel: %s", err)
		}
		if stream.Stream.Id != 0 {
			channelOnline = append(channelOnline, ChannelOnline{
				Title:     stream.Stream.Channel.Status,
				Channel:   stream.Stream.Channel.DisplayName,
				ChannelID: stream.Stream.Channel.Name,
				URL:       stream.Stream.Channel.Url,
				Game:      stream.Stream.Game,
				ThumbURL:  stream.Stream.Preview,
			})
		}
	}
	return channelOnline, nil
}

func (clientVideo ClientVideo) twGetVideo(subVideos []SubVideo) (_ []SubVideo, err error) {
	clientTW := twitch.NewClient(clientVideo.client)
	clientTW.ClientId = clientVideo.twClientID
	opt := &twitch.ListOptions{
		Limit:  50,
		Offset: 0,
	}

	fol, err := clientTW.Users.Follows(clientVideo.twUserName, opt)
	if err != nil {
		return subVideos, fmt.Errorf("GW!Twitch-Users-Follows %s", err)
	}

	for _, follow := range fol.Follows {
		videos, err := clientTW.Channels.Videos(follow.Channel.Name, opt)
		if err != nil {
			return subVideos, fmt.Errorf("GW!Twitch-Channels-Videos %s", err)
		}
		for _, video := range videos.Videos {
			twTime, err := time.Parse(time.RFC3339, video.RecordedAt)
			if err != nil {
				return subVideos, err
			}

			subVideos = append(subVideos, SubVideo{
				TypeSub:     "twitch",
				Title:       video.Title,
				Channel:     video.Channel.DisplayName,
				ChannelID:   video.Channel.Name,
				Game:        video.Game,
				Description: video.Description,
				URL:         video.Url,
				ThumbURL:    video.Preview,
				Date:        twTime.In(client.TimeZone),
				dateInt:     twTime.Unix(),
			})
		}
	}

	return subVideos, nil
}

func (clientVideo ClientVideo) ytGetVideo(subVideos []SubVideo) (_ []SubVideo, err error) {
	service, _ := youtube.New(clientVideo.client)
	call := service.Subscriptions.List("snippet").
		ChannelId(clientVideo.ytChannelID).
		MaxResults(50)
	response, _ := call.Do()

	for _, item := range response.Items {
		channelID := item.Snippet.ResourceId.ChannelId

		callVideo := service.Search.List("snippet").
			Q("").
			ChannelId(channelID).
			MaxResults(5).
			Order("date").
			Type("video")

		responseVideo, err := callVideo.Do()
		if err != nil {
			return subVideos, err
		}

		for _, video := range responseVideo.Items {
			ytTime, err := time.Parse(time.RFC3339, video.Snippet.PublishedAt)
			if err != nil {
				return subVideos, err
			}

			subVideos = append(subVideos, SubVideo{
				TypeSub:     "youtube",
				Title:       video.Snippet.Title,
				Description: video.Snippet.Description,
				Channel:     video.Snippet.ChannelTitle,
				ChannelID:   video.Snippet.ChannelId,
				Game:        "",
				URL:         "https://www.youtube.com/watch?v=" + video.Id.VideoId,
				ThumbURL:    video.Snippet.Thumbnails.High.Url,
				Date:        ytTime.In(client.TimeZone),
				dateInt:     ytTime.Unix(),
			})
		}
	}
	return subVideos, nil
}
