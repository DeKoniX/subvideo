package models

import (
	"fmt"

	"github.com/go-xorm/xorm"
	_ "github.com/lib/pq"
)

var (
	x   *xorm.Engine
	err error
)

func Init(host, port, username, password, dbname string) (err error) {
	pgurl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", username, password, host, port, dbname)
	x, err = xorm.NewEngine("postgres", pgurl)
	if err != nil {
		return err
	}
	err = x.Sync(new(User))
	if err != nil {
		return err
	}
	err = x.Sync(new(Subvideo))
	if err != nil {
		return err
	}
	// x.ShowSQL(true)
	return nil
}
