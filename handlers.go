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
	YtChannelID string `form:"yt_channel_id" binding:"Required"`
	TimeZone    string `form:"timezone" binding:"Required"`
}

func logoutHandler(ctx *macaron.Context) {
	ctx.SetCookie("username", "", -1)
	ctx.SetCookie("crypt", "", -1)
	ctx.Redirect("/")
}

func twOAuthHandler(ctx *macaron.Context) {
	code := ctx.Query("code")
	oauth := clientVideo.twClient.Auth(code)
	user, err := clientVideo.twClient.OAuthTest(oauth)
	if err != nil {
		log.Println("ERR OAUTH Twitch:", err)
		ctx.Redirect("/login")
	}
	timeNow := time.Now().UTC()
	hash := crypt(user.UserName, timeNow)

	user.TWOAuth = oauth
	user.Crypt = hash
	user.UpdatedAt = timeNow

	err = user.Insert()
	if err != nil {
		log.Panic(err)
	}
	user, err = models.SelectUserForUserName(user.UserName)
	if err != nil {
		log.Panic(err)
	}
	go runUser(user)

	ctx.SetCookie("username", user.UserName, time.Now().Add(time.Hour*24*30))
	ctx.SetCookie("crypt", hash, time.Now().Add(time.Hour*24*30))
	ctx.Redirect("/")
}

func loginHandler(ctx *macaron.Context) {
	u, _ := url.Parse("https://api.twitch.tv/kraken/oauth2/authorize")
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", clientVideo.twClient.ClientID)
	q.Set("scope", "user_read")
	q.Set("redirect_uri", config.Twitch.RedirectURI)
	u.RawQuery = q.Encode()

	ctx.Data["HeadURL"] = config.HeadURL
	ctx.Data["URL"] = u.String()
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
		channelOnline := clientVideo.twClient.GetOnline(user.TWOAuth)
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
		ytChannelID := changeUserForm.YtChannelID
		timezone := changeUserForm.TimeZone

		user.YTChannelID = ytChannelID
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
