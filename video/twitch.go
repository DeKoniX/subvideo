package video

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"errors"

	"github.com/DeKoniX/subvideo/models"
)

type TW struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	HTTPClient   *http.Client
}

func TWInit(clientID, clientSecret, redirectURI string) *TW {
	return &TW{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		HTTPClient:   &http.Client{},
		RedirectURI:  redirectURI,
	}
}

func (tw *TW) connect(url, oauth string) (body []byte, err error) {
	req, err := http.NewRequest("GET", "https://api.twitch.tv/kraken/"+url, nil)
	if err != nil {
		return body, err
	}
	req.Header.Add("Accept", "application/vnd.twitchtv.v5+json")
	req.Header.Add("Client-ID", tw.ClientID)
	if oauth != "" {
		req.Header.Add("Authorization", "OAuth "+oauth)
	}
	resp, err := tw.HTTPClient.Do(req)
	if err != nil {
		return body, err
	}
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return body, err
	}

	return body, nil
}

func (tw *TW) OAuthTest(accessToken string) (twChannelID, userName, avatarURL string, err error) {
	body, err := tw.connect("user", accessToken)
	if err != nil {
		return twChannelID, userName, avatarURL, err
	}

	type twJSON struct {
		DisplayName string `json:"display_name"`
		Name        string `json:"name"`
		Logo        string `json:"logo"`
		Error       string `json:"error"`
		Message     string `json:"message"`
	}

	var twjson twJSON

	err = json.Unmarshal(body, &twjson)
	if err != nil {
		return twChannelID, userName, avatarURL, err
	}

	if twjson.Error != "" {
		return twChannelID, userName, avatarURL, fmt.Errorf("ERR Twitch API: %s, %s", twjson.Error, twjson.Message)
	}

	twChannelID = twjson.Name
	userName = twjson.DisplayName
	avatarURL = twjson.Logo
	return twChannelID, userName, avatarURL, nil
}

func (tw *TW) Auth(code string) (accessToken string, err error) {
	resp, err := tw.HTTPClient.PostForm("https://api.twitch.tv/kraken/oauth2/token",
		url.Values{
			"client_id":     {tw.ClientID},
			"client_secret": {tw.ClientSecret},
			"grant_type":    {"authorization_code"},
			"redirect_uri":  {tw.RedirectURI},
			"code":          {code},
		},
	)
	if err != nil {
		return accessToken, err
	}
	type jsonTW struct {
		AccessToken string `json:"access_token"`
	}

	var jsontw jsonTW
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return accessToken, err
	}
	err = json.Unmarshal(body, &jsontw)
	if err != nil {
		return accessToken, err
	}

	return jsontw.AccessToken, nil
}

func (tw *TW) GetOnline(oauth string) (videos []models.Subvideo, err error) {
	body, err := tw.connect("streams/followed?limit=100&stream_type=live", oauth)
	if err != nil {
		return videos, err
	}

	type jsonTW struct {
		Streams []struct {
			Game      string `json:"game"`
			CreatedAt string `json:"created_at"`
			Preview   struct {
				Large string `json:"large"`
			}
			Channel struct {
				Status      string `json:"status"`
				DisplayName string `json:"display_name"`
				ID          int    `json:"_id"`
				URL         string `json:"url"`
			}
		}
	}

	var jsontw jsonTW
	err = json.Unmarshal(body, &jsontw)
	if err != nil {
		return videos, err
	}

	for _, stream := range jsontw.Streams {
		twTime, err := time.Parse(time.RFC3339, stream.CreatedAt)
		if err != nil {
			twTime = time.Now()
		}
		videos = append(videos, models.Subvideo{
			TypeSub:   "twitch-stream",
			Title:     stream.Channel.Status,
			Channel:   stream.Channel.DisplayName,
			ChannelID: strconv.Itoa(stream.Channel.ID),
			Game:      stream.Game,
			ThumbURL:  stream.Preview.Large,
			URL:       stream.Channel.URL,
			Length:    getLength(twTime),
		})
	}
	return videos, nil
}

func (tw *TW) GetVideos(oauth string) (videos []models.Subvideo, err error) {
	body, err := tw.connect("videos/followed?limit=100&broadcast_type=all", oauth)
	if err != nil {
		return videos, err
	}

	type jsonTW struct {
		Videos []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			URL         string `json:"url"`
			ID          string `json:"_id"`
			RecordedAt  string `json:"recorded_at"`
			Game        string `json:"game"`
			Length      int    `json:"length"`
			Preview     struct {
				Large string `json:"large"`
			}
			Channel struct {
				ID          int    `json:"_id"`
				DisplayName string `json:"display_name"`
			}
		}
	}

	var jsontw jsonTW
	err = json.Unmarshal(body, &jsontw)
	if err != nil {
		return videos, err
	}

	for _, video := range jsontw.Videos {
		twTime, err := time.Parse(time.RFC3339, video.RecordedAt)
		if err != nil {
			return videos, err
		}
		if video.Length > 300 {
			videos = append(videos, models.Subvideo{
				TypeSub:     "twitch",
				Title:       video.Title,
				Channel:     video.Channel.DisplayName,
				ChannelID:   strconv.Itoa(video.Channel.ID),
				Game:        video.Game,
				Description: video.Description,
				URL:         video.URL,
				VideoID:     video.ID,
				ThumbURL:    video.Preview.Large,
				Length:      video.Length,
				Date:        twTime.UTC(),
			})
		}
	}

	return videos, nil
}

func (tw *TW) GetChannel(oauth, channelID string) (video models.Subvideo, err error) {
	body, err := tw.connect("channels/"+channelID, oauth)
	if err != nil {
		return video, err
	}

	type jsonTW struct {
		Status      string `json:"status"`
		DisplayName string `json:"display_name"`
		Game        string `json:"game"`
		Name        string `json:"name"`
		URL         string `json:"url"`
		Error       string `json:"error"`
	}

	var jsontw jsonTW
	err = json.Unmarshal(body, &jsontw)
	if err != nil {
		return video, err
	}

	if jsontw.Error != "" {
		return video, errors.New("ERR: " + channelID + ": " + jsontw.Error)
	}

	return models.Subvideo{
		TypeSub:   "twitch-stream",
		Title:     jsontw.Status,
		Channel:   jsontw.DisplayName,
		ChannelID: jsontw.Name,
		Game:      jsontw.Game,
		URL:       jsontw.URL,
	}, nil
}
