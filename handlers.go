package main

import (
	"fmt"
	"log"
	"net/url"
	"strconv"
	"time"

	"github.com/DeKoniX/subvideo/models"

	"gopkg.in/macaron.v1"
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
	oauth, err := clientVideo.TWClient.Auth(code)
	if err != nil {
		log.Println("ERR OAUTH Twitch:", err)
		ctx.Redirect("/login")
	}
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
	user := currentUser(ctx.GetCookie("username"), ctx.GetCookie("crypt"))

	if user.UserName != "" {
		ctx.Redirect("/")
		return
	}

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

func searchHandler(ctx *macaron.Context) {
	var page int

	pageS := ctx.Req.FormValue("page")
	page, err := strconv.Atoi(pageS)
	if err != nil || page == 0 {
		page = 1
	}
	search := ctx.Req.FormValue("search")

	user := currentUser(ctx.GetCookie("username"), ctx.GetCookie("crypt"))

	if user.UserName != "" {
		var title string

		subVideos, count, err := clientVideo.SearchVideo(user, 42, page, search)
		if err != nil {
			log.Panicln(err)
		}
		pag := pagination(page, count, 42, "/search?search="+search+"&")
		if len(subVideos) == 0 {
			if pag.Previous == 0 {
				ctx.Redirect("/")
				return
			}
			ctx.Redirect("/search?search=" + search + "&page=" + strconv.Itoa(pag.Previous))
			return
		}
		title = fmt.Sprintf("Поиск по строке: %s", search)

		ctx.Data["HeadInfo"] = headInfo{Title: title, URL: config.HeadURL + ctx.Req.URL.String()[1:]}
		ctx.Data["Search"] = search
		ctx.Data["SubVideos"] = subVideos
		ctx.Data["User"] = user
		ctx.Data["SubVideo"] = models.Subvideo{}
		ctx.Data["Page"] = pag
		ctx.HTML(200, "search")
	} else {
		ctx.Redirect("/login")
		return
	}
}

func lastHandler(ctx *macaron.Context) {
	var page int

	pageS := ctx.Req.FormValue("page")
	page, err := strconv.Atoi(pageS)
	if err != nil || page == 0 {
		page = 1
	}
	channelID := ctx.Req.FormValue("channelID")

	user := currentUser(ctx.GetCookie("username"), ctx.GetCookie("crypt"))

	if user.UserName != "" {
		var title string

		subVideos, count, err := clientVideo.SortVideo(user, 42, channelID, page)
		if err != nil {
			log.Panicln(err)
		}
		pag := pagination(page, count, 42, "/last?channelID="+channelID+"&")

		if len(subVideos) == 0 {
			if pag.Previous == 0 {
				ctx.Redirect("/")
				return
			}
			ctx.Redirect("/last?channelID=" + channelID + "&page=" + strconv.Itoa(pag.Previous))
			return
		}
		title = fmt.Sprintf("%s последние видео", subVideos[0].Channel)

		ctx.Data["HeadInfo"] = headInfo{Title: title, URL: config.HeadURL + ctx.Req.URL.String()[1:]}
		ctx.Data["SubVideos"] = subVideos
		ctx.Data["User"] = user
		ctx.Data["SubVideo"] = models.Subvideo{}
		ctx.Data["Page"] = pag

		ctx.HTML(200, "last")
	} else {
		ctx.Redirect("/login")
		return
	}
}

func indexHandler(ctx *macaron.Context) {
	var page int

	pageS := ctx.Req.FormValue("page")
	page, err := strconv.Atoi(pageS)
	if err != nil || page == 0 {
		page = 1
	}
	user := currentUser(ctx.GetCookie("username"), ctx.GetCookie("crypt"))

	if user.UserName != "" {
		var title string

		// TODO: display panic error to user

		subVideos, count, err := clientVideo.SortVideo(user, 42, "", page)
		if err != nil {
			log.Panicln(err)
		}
		pag := pagination(page, count, 42, "/?")

		channelOnline, err := clientVideo.GetOnlineStreams(user)
		if err != nil {
			log.Panicln(err)
		}
		switch len(channelOnline) {
		case 1:
			title = fmt.Sprintf("Сейчас идет %d стрим", len(channelOnline))
		case 2, 3, 4:
			title = fmt.Sprintf("Сейчас идет %d стрима", len(channelOnline))
		case 0:
			title = fmt.Sprint("Стримов сейчас нет")
		default:
			title = fmt.Sprintf("Сейчас идет %d стримов", len(channelOnline))
		}
		if page != 1 {
			title += fmt.Sprintf(", страница %d", page)
		}

		ctx.Data["HeadInfo"] = headInfo{Title: title, URL: config.HeadURL + ctx.Req.URL.String()[1:]}
		ctx.Data["SubVideos"] = subVideos
		ctx.Data["ChannelOnline"] = channelOnline
		ctx.Data["User"] = user
		ctx.Data["SubVideo"] = models.Subvideo{}
		ctx.Data["Page"] = pag

		ctx.HTML(200, "index")
	} else {
		ctx.Redirect("/login")
		return
	}
}

func playHandler(ctx *macaron.Context) {
	typeVideo := ctx.Req.FormValue("type")
	idVideo := ctx.Req.FormValue("id")
	user := currentUser(ctx.GetCookie("username"), ctx.GetCookie("crypt"))
	if typeVideo == "twitch-stream" {
		subvideo, err := clientVideo.TWClient.GetChannel(user.TWOAuth, idVideo)
		if err != nil {
			log.Println(err)
			ctx.Redirect("/")
		}
		ctx.Data["SubVideo"] = subvideo
		ctx.Data["HeadInfo"] = headInfo{Title: subvideo.Title, URL: subvideo.URL, ImageURL: subvideo.ThumbURL, Description: subvideo.Description}
	} else {
		subvideo, err := models.SelectVideoForID(idVideo)
		if err != nil {
			log.Println(err)
			ctx.Redirect("/")
		}
		ctx.Data["SubVideo"] = subvideo
		embedDomain, _ := url.Parse(config.HeadURL)
		ctx.Data["HeadInfo"] = headInfo{Title: subvideo.Title, URL: subvideo.URL, ImageURL: subvideo.ThumbURL, Description: subvideo.Description, EmbedDomain: embedDomain.Hostname()}
	}
	ctx.Data["TypeVideo"] = typeVideo

	if user.UserName != "" {
		ctx.Data["User"] = user
	} else {
		ctx.Data["User"] = models.User{}
	}
	ctx.HTML(200, "play")
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
		ctx.Data["HeadInfo"] = headInfo{Title: title, URL: config.HeadURL + ctx.Req.URL.String()[1:]}
		ctx.Data["User"] = user
		ctx.Data["SubVideo"] = models.Subvideo{}
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
