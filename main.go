package main

import (
	"fmt"
	"time"

	"github.com/lornshark/shark/sharkapp"
	"github.com/lornshark/shark/sharkdb"
)

type MyApp struct {
	*sharkapp.App
}

type XAdminLog struct {
	AutoId     int       `gorm:"column:auto_id;primaryKey;autoIncrement" json:"auto_id"`
	CreateTime time.Time `gorm:"column:create_time" json:"create_time"`
	Id         int       `gorm:"column:id" json:"id"`
}

func main() {
	project := "kgame"
	name := "game-test"
	id := "1"
	options := sharkapp.NewOptionWithRedis("dev", project, name, id, "192.168.191.100", "6379", "CEki57pxTJyYaLD")
	options.WithDb(nil).WithRedis(nil).WithKafka(nil).WithElastic(nil).
		WithMongodb(nil).WithRabbitmq(nil).WithRisingwave(nil)
	app, err := sharkapp.New(options)
	if err != nil {
		panic(err)
	}

	scaner := sharkdb.NewTableScan[XAdminLog]().PageSize(100).OrderAsc("create_time").OrderAsc("auto_id").PageSize(10)
	var last *XAdminLog = &XAdminLog{CreateTime: time.UnixMilli(1779276409032), AutoId: 1441151880758588731}
	db := app.Db.Table("x_admin_log")
	results, err := scaner.Prev(db, last)
	if err != nil {
		panic(err)
	}
	for _, result := range results {
		fmt.Println(result.Id, result.AutoId, result.CreateTime.UnixMilli())
	}
	fmt.Println("==========")
	last = nil
	count := 0
	for i := 0; i < 2; i++ {
		db := app.Db.Table("x_admin_log")
		results, err := scaner.Next(db, last)
		if err != nil {
			panic(err)
		}
		if len(results) == 0 {
			break
		}
		last = &results[len(results)-1]
		count += len(results)
		for _, result := range results {
			fmt.Println(result.Id, result.AutoId, result.CreateTime.UnixMilli())
		}
		fmt.Println("==========")
	}
	fmt.Printf("total count: %d\n", count)
	app.Hunt()
}
