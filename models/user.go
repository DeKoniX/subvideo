package models

import (
	"errors"
	"time"
)

type User struct {
	Id             int64
	YTChannelID    string    `xorm:"'yt_channel_id'"`
	TWChannelID    string    `xorm:"'tw_channel_id'"`
	TWOAuth        string    `xorm:"'tw_oauth'"`
	YTOAuth        string    `xorm:"'yt_oauth'"`
	YTRefreshToken string    `xorm:"'yt_refresh_token'"`
	YTExpiry       time.Time `xorm:"'yt_expiry'"`
	UserName       string    `xorm:"notnull index 'username'"`
	AvatarURL      string    `xorm:"'avatar_url'"`
	Crypt          string    `xorm:"'crypt'"`
	TimeZone       string    `xorm:"'timezone'"`
	CreatedAt      time.Time `xorm:"created"`
	UpdatedAt      time.Time `xorm:"'updated_at'"`
}

func (user User) Insert() error {
	b, err := x.Get(&User{UserName: user.UserName})
	if err != nil {
		return err
	}
	if b == false {
		_, err = x.Insert(&user)
		if err != nil {
			return err
		}
	} else {
		_, err = x.Update(&user, User{UserName: user.UserName})
		if err != nil {
			return err
		}
	}
	return nil
}

func SelectUserForUserName(name string) (user User, err error) {
	b, err := x.Where("username = ?", name).Get(&user)
	if err != nil {
		return user, err
	}
	if b == false {
		return user, errors.New("No!")
	}
	return user, err
}

func SelectUsers() (users []User, err error) {
	err = x.Find(&users)
	return users, err
}

func DeleteUserWhereInterval(day int) (err error) {
	duration := time.Hour * time.Duration(24*day)
	dateInterval := time.Now().Add(-duration)
	dateInterval.Format(time.RFC3339)
	_, err = x.Where("updated_at<?", dateInterval).Delete(&User{})
	return err
}
