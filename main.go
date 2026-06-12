package main

import (
	"os"
	"path"
	"time"

	"github.com/lornshark/shark/sharkapp"
	"github.com/lornshark/shark/sharkdb"
)

// @title game-demo API
// @version 1.0

// @BasePath  /api

// @securityDefinitions.apiKey ApiKeyAuth
// @in header
// @name x-token

// go get -u github.com/swaggo/swag/cmd/swag
// go install github.com/swaggo/swag/cmd/swag@v1.16.4
// swag v1.16.4

// swag init --parseDependency -g  main.go

func main() {
	options := sharkapp.NewOption("kgame", "game-test")
	app, err := sharkapp.New(options)
	if err != nil {
		panic(err)
	}
	app.Hunt(&Test{app: app})
}

type Test struct {
	app *sharkapp.App
}

type XAdminLog struct {
	AutoID     int64     `gorm:"column:auto_id;primaryKey" json:"auto_id"`
	ID         int64     `gorm:"column:id;not null;comment:id" json:"id"` // id
	MerchantID int32     `gorm:"column:merchant_id;not null" json:"merchant_id"`
	UserID     int32     `gorm:"column:user_id" json:"user_id"`
	Account    string    `gorm:"column:account;not null" json:"account"`
	ReqPath    string    `gorm:"column:req_path;not null" json:"req_path"`
	ReqData    string    `gorm:"column:req_data;not null" json:"req_data"`
	IP         string    `gorm:"column:ip;not null" json:"ip"`
	CreateTime time.Time `gorm:"column:create_time;not null;default:CURRENT_TIMESTAMP(3)" json:"create_time"`
}

func (t *Test) Start() {
	db := t.app.Db.Table("x_admin_log")
	scanner := sharkdb.NewTableScan[XAdminLog]().PageSize(10000).OrderAsc("create_time").OrderAsc("auto_id")
	x, err := scanner.Export(db, "test", []any{"auto_id", "id", "merchant_id", "user_id", "account", "req_path", "req_data", "ip", "create_time"}, func(log XAdminLog) []any {
		return []any{log.AutoID, log.ID, log.MerchantID, log.UserID, log.Account, log.ReqPath, log.ReqData, log.IP, log.CreateTime}
	})
	if err != nil {
		panic(err)
	}
	println("导出成功:", path.Join(os.TempDir(), x))
}
