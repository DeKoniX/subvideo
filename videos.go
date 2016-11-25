package main

import (
	"fmt"
	"log"
	"net/http"
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
}

type ChannelOnline struct {
	Title     string
	Channel   string
	ChannelID string
	URL       string
	Game      string
	ThumbURL  string
}

type ClientVideo struct {
	client      *http.Client
	twClientID  string
	twUserName  string
	ytChannelID string
	TimeZone    *time.Location
	DataBase    DB
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
	subVideos, err = client.DataBase.Select(n)
	if err != nil {
		log.Fatal(err)
		return subVideos, err
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

func (clientVideo ClientVideo) TWGetVideo() (err error) {
	clientTW := twitch.NewClient(clientVideo.client)
	clientTW.ClientId = clientVideo.twClientID
	opt := &twitch.ListOptions{
		Limit:  50,
		Offset: 0,
	}

	fol, err := clientTW.Users.Follows(clientVideo.twUserName, opt)
	if err != nil {
		return err
	}

	for _, follow := range fol.Follows {
		videos, err := clientTW.Channels.Videos(follow.Channel.Name, opt)
		if err != nil {
			return err
		}
		for _, video := range videos.Videos {
			twTime, err := time.Parse(time.RFC3339, video.RecordedAt)
			if err != nil {
				return err
			}

			err = clientVideo.DataBase.Insert(
				"twitch",
				video.Title,
				video.Channel.DisplayName,
				video.Channel.Name,
				video.Game,
				video.Description,
				video.Url,
				video.Preview,
				twTime.In(client.TimeZone),
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (clientVideo ClientVideo) YTGetVideo() (err error) {
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
			return err
		}

		for _, video := range responseVideo.Items {
			ytTime, err := time.Parse(time.RFC3339, video.Snippet.PublishedAt)
			if err != nil {
				return err
			}

			err = clientVideo.DataBase.Insert(
				"youtube",
				video.Snippet.Title,
				video.Snippet.ChannelTitle,
				video.Snippet.ChannelId,
				"",
				video.Snippet.Description,
				"https://www.youtube.com/watch?v="+video.Id.VideoId,
				video.Snippet.Thumbnails.High.Url,
				ytTime.In(client.TimeZone),
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
