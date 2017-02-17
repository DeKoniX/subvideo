package main

import (
	"fmt"
	"log"
	"net/url"
	"strconv"
	"time"

	"github.com/DeKoniX/subvideo/models"

	macaron "gopkg.in/macaron.v1"
)

type ChangeUserForm struct {
	TimeZone string `form:"timezone" binding:"Required"`
}

func logoutHandler(ctx *macaron.Context) {
	ctx.SetCookie("username", "", -1)
	ctx.SetCookie("crypt", "", -1)
	ctx.Redirect("/")
}

func twOAuthHandler(ctx *macaron.Context) {
	code := ctx.Query("code")
	oauth := clientVideo.TWClient.Auth(code)
	twChannelID, userName, avatarURL, err := clientVideo.TWClient.OAuthTest(oauth)
	if err != nil {
		log.Println("ERR OAUTH Twitch:", err)
		ctx.Redirect("/login")
	}
	user := currentUser(ctx.GetCookie("username"), ctx.GetCookie("crypt"))
	if user.UserName == "" {
		user, _ = models.SelectUserForUserName(userName)
		if user.UserName == "" {
			user.UserName = userName
		}
	}

	user.TWChannelID = twChannelID
	user.AvatarURL = avatarURL

	timeNow := time.Now().UTC()
	hash := crypt(user.UserName, timeNow)

	user.TWOAuth = oauth
	user.Crypt = hash
	user.UpdatedAt = timeNow

	err = user.Insert()
	if err != nil {
		log.Println("ERR USER ADD:", err)
		ctx.Redirect("/login")
	}
	go runUser(user)

	ctx.SetCookie("username", user.UserName, time.Now().Add(time.Hour*24*30))
	ctx.SetCookie("crypt", hash, time.Now().Add(time.Hour*24*30))
	ctx.Redirect("/")
}

func ytOAuthHandler(ctx *macaron.Context) {
	code := ctx.Query("code")
	token := clientVideo.YTClient.Auth(code)
	ytChannelID, userName, avatarURL, err := clientVideo.YTClient.OAuthTest(token)
	if err != nil {
		log.Println("ERR OAUTH YouTube:", err)
		ctx.Redirect("/login")
	}
	user := currentUser(ctx.GetCookie("username"), ctx.GetCookie("crypt"))
	if user.UserName == "" {
		user, _ = models.SelectUserForUserName(userName)
		if user.UserName == "" {
			user.UserName = userName
		}
	}
	user.YTChannelID = ytChannelID
	user.AvatarURL = avatarURL

	timeNow := time.Now().UTC()
	hash := crypt(user.UserName, timeNow)

	user.YTOAuth = token.AccessToken
	user.YTRefreshToken = token.RefreshToken
	user.YTExpiry = token.Expiry
	user.Crypt = hash
	user.UpdatedAt = timeNow

	err = user.Insert()
	if err != nil {
		log.Println("ERR USER ADD:", err)
		ctx.Redirect("/login")
	}

	user, _ = models.SelectUserForUserName(user.UserName)

	go runUser(user)

	ctx.SetCookie("username", user.UserName, time.Now().Add(time.Hour*24*30))
	ctx.SetCookie("crypt", hash, time.Now().Add(time.Hour*24*30))
	ctx.Redirect("/")
}

func loginHandler(ctx *macaron.Context) {
	u, _ := url.Parse("https://api.twitch.tv/kraken/oauth2/authorize")
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", clientVideo.TWClient.ClientID)
	q.Set("scope", "user_read")
	q.Set("redirect_uri", config.Twitch.RedirectURI)
	u.RawQuery = q.Encode()

	ctx.Data["HeadURL"] = config.HeadURL
	ctx.Data["TwitchURL"] = u.String()
	ctx.Data["YouTubeURL"] = clientVideo.YTClient.URL
	ctx.HTML(200, "login")
}

func lastHandler(ctx *macaron.Context) {
	var page, pageNext, pageLast int

	pageS := ctx.Req.FormValue("page")
	page, err := strconv.Atoi(pageS)
	if err != nil || page == 0 {
		page = 1
	}
	pageNext = page + 1
	pageLast = page - 1

	channelID := ctx.Req.FormValue("channelID")

	user := currentUser(ctx.GetCookie("username"), ctx.GetCookie("crypt"))

	if user.UserName != "" {
		var title string
		type pageStruct struct {
			Page int
			Next int
			Last int
		}

		subVideos, err := clientVideo.SortVideo(user, 42, channelID, page)
		if len(subVideos) == 0 {
			if pageLast == 0 {
				ctx.Redirect("/")
				return
			}
			ctx.Redirect("/last?channelID=" + channelID + "&page=" + strconv.Itoa(pageLast))
			return
		}
		if err != nil {
			log.Panicln(err)
		}
		title = fmt.Sprintf("%s последние видео", subVideos[0].Channel)

		ctx.Data["Title"] = title
		ctx.Data["SubVideos"] = subVideos
		ctx.Data["User"] = user
		ctx.Data["Page"] = pageStruct{
			Page: page,
			Next: pageNext,
			Last: pageLast,
		}
		ctx.HTML(200, "last")
	} else {
		ctx.Redirect("/login")
		return
	}
}

func indexHandler(ctx *macaron.Context) {
	var page, pageNext, pageLast int

	pageS := ctx.Req.FormValue("page")
	page, err := strconv.Atoi(pageS)
	if err != nil || page == 0 {
		page = 1
	}
	pageNext = page + 1
	pageLast = page - 1

	user := currentUser(ctx.GetCookie("username"), ctx.GetCookie("crypt"))

	if user.UserName != "" {
		var title string
		type pageStruct struct {
			Page int
			Next int
			Last int
		}

		subVideos, err := clientVideo.SortVideo(user, 42, "", page)
		for _, s := range subVideos {
			s.Date = timeZone(s.Date, user.TimeZone)
		}
		if err != nil {
			log.Panicln(err)
		}
		channelOnline := clientVideo.TWClient.GetOnline(user.TWOAuth)
		switch len(channelOnline) {
		case 1:
			title = fmt.Sprintf("сейчас идет %d стрим", len(channelOnline))
		case 2, 3, 4:
			title = fmt.Sprintf("сейчас идет %d стрима", len(channelOnline))
		default:
			title = fmt.Sprintf("сейчас идет %d стримов", len(channelOnline))
		}
		if page != 1 {
			title += fmt.Sprintf(", страница %d", page)
		}

		ctx.Data["Title"] = title
		ctx.Data["SubVideos"] = subVideos
		ctx.Data["ChannelOnline"] = channelOnline
		ctx.Data["User"] = user
		ctx.Data["Page"] = pageStruct{
			Page: page,
			Next: pageNext,
			Last: pageLast,
		}

		ctx.HTML(200, "index")
	} else {
		ctx.Redirect("/login")
		return
	}
}

func userHandler(ctx *macaron.Context) {
	user := currentUser(ctx.GetCookie("username"), ctx.GetCookie("crypt"))

	if user.UserName != "" {
		u, _ := url.Parse("https://api.twitch.tv/kraken/oauth2/authorize")
		q := u.Query()
		q.Set("response_type", "code")
		q.Set("client_id", clientVideo.TWClient.ClientID)
		q.Set("scope", "user_read")
		q.Set("redirect_uri", config.Twitch.RedirectURI)
		u.RawQuery = q.Encode()

		ctx.Data["TwitchURL"] = u.String()
		ctx.Data["YouTubeURL"] = clientVideo.YTClient.URL

		var title string

		title = fmt.Sprintf("Настройки пользователя %s", user.UserName)
		ctx.Data["Title"] = title
		ctx.Data["User"] = user
		ctx.Data["TimeZones"] = getTimeZones()
		ctx.HTML(200, "user")
	} else {
		ctx.Redirect("/login")
	}
}

func userChangeHandler(ctx *macaron.Context, changeUserForm ChangeUserForm) {
	login := false

	user := currentUser(ctx.GetCookie("username"), ctx.GetCookie("crypt"))
	if user.UserName != "" {
		login = true
	}

	if login {
		timezone := changeUserForm.TimeZone
		user.TimeZone = timezone
		err := user.Insert()
		if err != nil {
			log.Panic(err)
		}

		user = currentUser(ctx.GetCookie("username"), ctx.GetCookie("crypt"))
		go runUser(user)
		ctx.Redirect("/")
	} else {
		ctx.Redirect("/login")
	}
}
