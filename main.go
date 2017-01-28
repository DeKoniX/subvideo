package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/go-macaron/binding"
	"github.com/go-macaron/gzip"
	"github.com/shaoshing/train"

	"strconv"

	macaron "gopkg.in/macaron.v1"
	yaml "gopkg.in/yaml.v2"
)

type configYML struct {
	YouTube struct {
		DeveloperKey string `yaml:"developerkey"`
	}
	Twitch struct {
		ClientID     string `yaml:"clientid"`
		ClientSecret string `yaml:"clientsecret"`
		RedirectURI  string `yaml:"redirecturi"`
	}
	DataBase struct {
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		DBname   string `yaml:"dbname"`
		UserName string `yaml:"username"`
		Password string `yaml:"password"`
	}
	Secret  string `yaml:"secret"`
	HeadURL string `yaml:"headurl"`
}

var config configYML
var clientVideo ClientVideo

type ChangeUserForm struct {
	YtChannelID string `form:"yt_channel_id" binding:"Required"`
	TimeZone    string `form:"timezone" binding:"Required"`
}

func main() {
	err := getConfig()
	if err != nil {
		log.Panic(err)
	}

	clientVideo = InitClientVideo(
		config.Twitch.ClientID,
		config.Twitch.ClientSecret,
		config.YouTube.DeveloperKey,
	)

	clientVideo.dataBase, err = DBInit(
		config.DataBase.Host,
		config.DataBase.Port,
		config.DataBase.UserName,
		config.DataBase.Password,
		config.DataBase.DBname,
	)
	if err != nil {
		log.Fatal("DB ERR: ", err)
	}
	go runTime()

	m := macaron.Classic()
	m.Use(macaron.Renderer(macaron.RenderOptions{
		Funcs: []template.FuncMap{map[string]interface{}{
			"javascript_tag":            train.JavascriptTag,
			"stylesheet_tag":            train.StylesheetTag,
			"stylesheet_tag_with_param": train.StylesheetTagWithParam,
			"split":                     split,
			"timeZone":                  timeZone,
			"videoLen":                  videoLen,
		}},
	}))
	m.Use(macaron.Static("public"))
	m.Use(gzip.Gziper())

	m.Get("/", indexHandler)
	m.Get("/last", lastHandler)
	m.Get("/oauth/twitch", twOAuthHandler)
	m.Get("/login", loginHandler)
	m.Get("/logout", logoutHandler)
	m.Combo("/user").
		Get(userHandler).
		Post(binding.Bind(ChangeUserForm{}), userChangeHandler)

	mux := http.NewServeMux()
	mux.Handle("/", m)
	mux.Handle("/last", m)
	mux.Handle("/login", m)
	mux.Handle("/user", m)
	train.ConfigureHttpHandler(mux)

	log.Println("Server is running...")
	log.Println(http.ListenAndServe(":8181", mux))
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
		log.Println(err)
		ctx.Redirect("/login")
	}
	date := time.Now().UTC()
	hash := crypt(user.UserName, date)

	err = clientVideo.dataBase.InsertUser(
		user.YTChannelID,
		user.TWChannelID,
		oauth,
		user.UserName,
		user.AvatarURL,
		user.TimeZone,
		hash,
		date,
	)
	if err != nil {
		log.Panic(err)
	}
	users, err := clientVideo.dataBase.SelectUserForUserName(user.UserName)
	if err != nil {
		log.Panic(err)
	}
	go runUser(users[0])

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

func currentUser(username, hash string) User {
	if username == "" {
		return User{}
	}
	users, err := clientVideo.dataBase.SelectUserForUserName(username)
	if err != nil {
		return User{}
	}
	if len(users) == 0 {
		return User{}
	}

	if cryptTest(username, hash, users[0].Date.UTC()) {
		return users[0]
	}
	return User{}
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
	var login bool

	user := currentUser(ctx.GetCookie("username"), ctx.GetCookie("crypt"))
	if user.UserName != "" {
		login = true
	}

	if login {
		ytChannelID := changeUserForm.YtChannelID
		timezone := changeUserForm.TimeZone
		// ytChannelID := ctx.Req.FormValue("yt_channel_id")
		// timezone := ctx.Req.FormValue("timezone")

		err := clientVideo.dataBase.InsertUser(
			ytChannelID,
			user.TWChannelID,
			user.TWOAuth,
			user.UserName,
			user.AvatarURL,
			timezone,
			user.Crypt,
			user.Date,
		)
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

func runUser(user User) {
	log.Println("RUN User: ", user.UserName)
	err := clientVideo.YTGetVideo(user)
	if err != nil {
		log.Println("ERR YT: ", err)
	}
	err = clientVideo.TWGetVideo(user)
	if err != nil {
		log.Println("ERR TW: ", err)
	}
}

func runTime() {
	var err error
	run := true

	users, err := clientVideo.dataBase.SelectUsers()
	if err != nil {
		log.Println("ERR Users get: ", err)
	}

	log.Println("This RUN groutine")
	for _, user := range users {

		err = clientVideo.YTGetVideo(user)
		if err != nil {
			log.Println("ERR YT: ", err)
		}
		err = clientVideo.TWGetVideo(user)
		if err != nil {
			log.Println("ERR TW: ", err)
		}
	}

	for {
		if time.Now().Minute()%30 == 0 && run {
			log.Println("RUN groutine")
			run = false
			users, err = clientVideo.dataBase.SelectUsers()
			if err != nil {
				log.Println("ERR Users get: ", err)
			}
			for _, user := range users {
				err := clientVideo.YTGetVideo(user)
				if err != nil {
					log.Println("ERR YT: ", err)
				}
				err = clientVideo.TWGetVideo(user)
				if err != nil {
					log.Println("ERR TW: ", err)
				}
			}
			if time.Now().Minute() == 0 {
				err = clientVideo.dataBase.DeleteVideoWhereInterval(10)
				if err != nil {
					log.Println("ERR Clear Video: ", err)
				}
				err = clientVideo.dataBase.DeleteUserWhereInterval(30)
				if err != nil {
					log.Println("ERR Clear Users: ", err)
				}
			}
		} else {
			run = true
		}

		time.Sleep(time.Second * 30)
	}
}

func getConfig() (err error) {
	dat, err := ioutil.ReadFile("subvideo.yml")
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(dat, &config)
	if err != nil {
		return err
	}
	return nil
}

func crypt(username string, dateChange time.Time) string {
	h := md5.New()
	io.WriteString(h, username)
	io.WriteString(h, config.Secret)
	io.WriteString(h, dateChange.Format(time.Stamp))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func cryptTest(username, hash string, dateChange time.Time) bool {
	h := md5.New()
	io.WriteString(h, username)
	io.WriteString(h, config.Secret)
	io.WriteString(h, dateChange.Format(time.Stamp))
	thisHash := fmt.Sprintf("%x", h.Sum(nil))
	return thisHash == hash
}

func split(a, b int) bool {
	return a%b == 0
}

func timeZone(t time.Time, tz string) (timeString string) {
	timezone, _ := time.LoadLocation(tz)
	ttz := t.In(timezone)
	timeString = ttz.Format("02-01-06 ------ 15:04")
	return timeString
}

type timeZones []struct {
	Offset int    `json:"offset"`
	UTC    string `json:"utc"`
}

func getTimeZones() (timeZones timeZones) {
	dat, _ := ioutil.ReadFile("timezones.json")
	json.Unmarshal(dat, &timeZones)

	return timeZones
}

func videoLen(len int) (strLength string) {
	var hour, min, second int
	if len > 60 {
		min = len / 60
		second = len % 60

		if min > 59 {
			hour = min / 60
			min = min % 60

			strLength = fmt.Sprintf("Часов: %d, Минуты: %d, ", hour, min)
		} else {
			strLength = fmt.Sprintf("Минуты: %d, ", min)
		}
		strLength = strLength + fmt.Sprintf("Секунды: %d", second)
	} else {
		strLength = fmt.Sprintf("Секунды: %d", len)
	}

	return strLength
}
