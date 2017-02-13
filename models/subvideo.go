package models

import "time"

type Subvideo struct {
	Id          int64
	TypeSub     string    `xorm:"'type'"`
	Title       string    `xorm:"'title'"`
	Channel     string    `xorm:"'channel'"`
	ChannelID   string    `xorm:"index 'channel_id'"`
	Game        string    `xorm:"'game'"`
	Description string    `xorm:"text 'description'"`
	URL         string    `xorm:"'url'"`
	ThumbURL    string    `xorm:"'thumb_url'"`
	Length      int       `xorm:"'length'"`
	Date        time.Time `xorm:"'date'"`
	UserID      int64     `xorm:"notnull index 'user_id'"`
	CreatedAt   time.Time `xorm:"created"`
	UpdatedAt   time.Time `xorm:"updated"`
}

func (subvideo Subvideo) Insert() (err error) {
	b, err := x.Get(&Subvideo{URL: subvideo.URL, UserID: subvideo.UserID})
	if err != nil {
		return err
	}
	if b == false {
		_, err = x.Insert(&subvideo)
		if err != nil {
			return err
		}
	} else {
		_, err = x.Update(&subvideo, Subvideo{URL: subvideo.URL, UserID: subvideo.UserID})
		if err != nil {
			return err
		}
	}
	return nil
}

func SelectVideo(userID, n int, channelID string, page int) (subvideos []Subvideo, err error) {
	if channelID == "" {
		err = x.Where("user_id = ?", userID).
			Desc("date").
			Limit(n, page*n-n).
			Find(&subvideos)
	} else {
		err = x.Where("user_id = ? AND channel_id = ?", userID, channelID).
			Desc("date").
			Limit(n, page*n-n).
			Find(&subvideos)
	}
	return subvideos, err
}

func DeleteVideoWhereInterval(day int) (err error) {
	duration := time.Hour * time.Duration(24*day)
	dateInterval := time.Now().Add(-duration)
	dateInterval.Format(time.RFC3339)
	_, err = x.Where("date<?", dateInterval).Delete(&Subvideo{})
	return err
}
