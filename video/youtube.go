package video

import (
	"context"
	"log"

	"strings"
	"time"

	"errors"

	"github.com/DeKoniX/subvideo/models"
	"golang.org/x/oauth2"
	"google.golang.org/api/plus/v1"
	"google.golang.org/api/youtube/v3"
)

type YT struct {
	context   context.Context
	oauthConf *oauth2.Config
	URL       string
}

func YTInit(clientID, clientSecret, redirectURL string) *YT {
	conf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{youtube.YoutubeReadonlyScope, plus.UserinfoProfileScope},
		RedirectURL:  redirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://accounts.google.com/o/oauth2/token",
		},
	}
	return &YT{
		context:   context.Background(),
		oauthConf: conf,
		URL:       conf.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.ApprovalForce),
	}
}

func (yt *YT) Auth(code string) *oauth2.Token {
	tok, _ := yt.oauthConf.Exchange(yt.context, code)
	return tok
}

func (yt *YT) OAuthTest(token *oauth2.Token) (ytChannelID, userName, avatarURL string, err error) {
	client := yt.oauthConf.Client(yt.context, token)
	plusService, err := plus.New(client)
	youtubeService, err := youtube.New(client)
	if err != nil {
		return ytChannelID, userName, avatarURL, err
	}

	person, err := plusService.People.Get("me").Do()
	if err != nil {
		return ytChannelID, userName, avatarURL, err
	}

	channel, err := youtubeService.Channels.List("id").Mine(true).Do()
	if err != nil {
		return ytChannelID, userName, avatarURL, err
	}

	userName = person.Nickname
	if userName == "" {
		userName = person.DisplayName
	}

	if channel.Items[0].Id == "" || userName == "" {
		return ytChannelID, userName, avatarURL, errors.New("No username or ytID: UserName: " + userName + " ytID: " + channel.Items[0].Id)
	}
	return channel.Items[0].Id, userName, person.Image.Url, nil
}

func (yt *YT) GetVideos(user models.User) (videos []models.Subvideo, err error) {
	token := oauth2.Token{AccessToken: user.YTOAuth, RefreshToken: user.YTRefreshToken, Expiry: user.YTExpiry, TokenType: "Bearer"}

	tokenSource := yt.oauthConf.TokenSource(yt.context, &token)
	updateToken, err := tokenSource.Token()
	if err != nil {
		return videos, err
	}

	if token.AccessToken != updateToken.AccessToken {
		user.YTOAuth = updateToken.AccessToken
		user.YTRefreshToken = updateToken.RefreshToken
		user.YTExpiry = updateToken.Expiry
		user.Insert()
	}

	if time.Now().After(user.YTExpiry) {
		log.Println("Очищаю токен", user.UserName)
		user.YTOAuth = ""
		user.YTRefreshToken = ""
		user.Insert()
	}

	client := oauth2.NewClient(yt.context, tokenSource)

	service, err := youtube.New(client)
	if err != nil {
		return videos, err
	}
	repeat := true
	pageToken := ""
	for repeat == true {
		repeat = false
		call := service.Subscriptions.List("snippet").Mine(true).MaxResults(50).PageToken(pageToken)
		response, err := call.Do()
		if err != nil {
			return videos, err
		}
		if response.NextPageToken != "" {
			pageToken = response.NextPageToken
			repeat = true
		}
		if err != nil {
			return videos, err
		}

		for _, item := range response.Items {
			var ids string
			channelID := item.Snippet.ResourceId.ChannelId

			callSearchVideos := service.Search.List("snippet").
				Q("").
				ChannelId(channelID).
				MaxResults(5).
				Order("date").
				Type("video")

			responseSearchVideos, err := callSearchVideos.Do()
			if err != nil {
				return videos, err
			}

			for _, videoSearch := range responseSearchVideos.Items {
				ids += videoSearch.Id.VideoId + ","
			}
			callVideos := service.Videos.List("snippet,contentDetails").Id(ids)
			responseVideos, err := callVideos.Do()
			if err != nil {
				return videos, err
			}

			for _, video := range responseVideos.Items {
				ytTime, err := time.Parse(time.RFC3339, video.Snippet.PublishedAt)
				if err != nil {
					return videos, err
				}
				durationVideo, err := time.ParseDuration(strings.ToLower(video.ContentDetails.Duration[2:]))
				if err != nil {
					return videos, err
				}

				videos = append(videos, models.Subvideo{
					TypeSub:     "youtube",
					Title:       video.Snippet.Title,
					Channel:     video.Snippet.ChannelTitle,
					ChannelID:   video.Snippet.ChannelId,
					Description: video.Snippet.Description,
					VideoID:     video.Id,
					URL:         "https://www.youtube.com/watch?v=" + video.Id,
					ThumbURL:    video.Snippet.Thumbnails.High.Url,
					Length:      int(durationVideo.Seconds()),
					Date:        ytTime.UTC(),
				})
			}
		}
	}

	return videos, nil
}
