package main

import (
	"context"
	"os"
	"shark/sharkauth"
	"shark/sharkdb"
	"shark/sharkredis"
	"shark/sharkrw"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
	}
	encoderConfig.CallerKey = "caller"
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
	consoleCore := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zapcore.DebugLevel)
	core := zapcore.NewTee(consoleCore)
	logger := zap.New(core, zap.AddCaller())

	db, err := sharkdb.NewDb(logger, &sharkdb.Config{
		Host:     "192.168.191.100",
		Port:     4000,
		User:     "root",
		Password: "CEki57pxTJyYaLD",
		Database: "kgame",
	})
	if err != nil {
		panic(err)
	}
	if err := db.WithContext(context.Background()).Exec("SELECT 1").Error; err != nil {
		panic(err)
	}
	logger.Info("Successfully connected to db and executed a query with sharksql.")

	rw, err := sharkrw.New(logger, &sharkrw.Config{
		Host:     "192.168.191.100",
		Port:     4566,
		User:     "root",
		Password: "CEki57pxTJyYaLD",
		Database: "kgame",
	})
	if err != nil {
		panic(err)
	}
	if err := rw.WithContext(context.Background()).Exec("SELECT 1").Error; err != nil {
		panic(err)
	}
	logger.Info("Successfully connected to rw and executed a query with sharkrw.")

	sharkauth := sharkauth.NormalizeAuthTree([]*sharkauth.AuthNode{
		{
			Name: "root",
			Children: []*sharkauth.AuthNode{
				{
					Name: "child1",
					Urls: []string{"/api/child1"},
				},
				{
					Name: "child2",
					Urls: []string{"/api/child2"},
				},
			},
		},
	}, 1)
	logger.Sugar().Infof("Normalized Auth Tree: %+v", sharkauth)
	rds, err := sharkredis.New(&sharkredis.Config{
		Host:     "192.168.191.100",
		Port:     0,
		Password: "CEki57pxTJyYaLD",
	})
	if err != nil {
		panic(err)
	}
	_ = rds
	logger.Info("Successfully connected to redis with sharkredis.")
}
