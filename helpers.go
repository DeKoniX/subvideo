package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/DeKoniX/subvideo/models"
)

func currentUser(username, hash string) (user models.User) {
	if username == "" {
		return user
	}
	user, err := models.SelectUserForUserName(username)
	if err != nil {
		return models.User{}
	}
	if cryptTest(username, hash, user.UpdatedAt.UTC()) {
		return user
	}
	return models.User{}
}

func split(a, b int) bool {
	return a%b == 0
}

func timeZone(t time.Time, tz string) (ttz time.Time) {
	timezone, _ := time.LoadLocation(tz)
	ttz = t.In(timezone)
	return ttz
}

func getTime(t time.Time) (timeString string) {
	timeString = t.Format("02-01-06 ------ 15:04")
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

type navMenuStruct struct {
	User     models.User
	SubVideo models.Subvideo
}

func navMenu(user models.User, subvideo models.Subvideo) navMenuStruct {
	return navMenuStruct{User: user, SubVideo: subvideo}
}

type headInfo struct {
	Title       string
	Description string
	URL         string
	ImageURL    string
}

type Metrics struct {
	Yandex int
	Google string
}

func metrics() (met Metrics) {
	met.Yandex = config.Metrics.Yandex
	met.Google = config.Metrics.Google
	return met
}
