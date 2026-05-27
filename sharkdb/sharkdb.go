package sharkdb

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Config struct {
	Host     string `json:"host"`     // 连接地址
	Port     int    `json:"port"`     // 连接端口
	User     string `json:"user"`     // 连接用户名
	Password string `json:"password"` // 连接密码
	Database string `json:"database"` // 连接数据库名称
	Tls      string `json:"tls"`      // 连接使用的 TLS 配置，默认值为 "" 表示不使用 TLS
}

func NewDb(logger *zap.Logger, config *Config) (*gorm.DB, error) {
	if config == nil {
		return nil, fmt.Errorf("config required")
	}
	dsn := "%v:%v@tcp(%v:%v)/%v?charset=utf8mb4&parseTime=True&loc=Local"
	dsn = fmt.Sprintf(dsn, config.User, config.Password, config.Host, config.Port, config.Database)
	if config.Tls != "" {
		dsn += "&tls=tidb"
		mysqldriver.RegisterTLSConfig("tidb", &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: config.Host,
		})
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: &log{
			logger: logger,
		},
	})
	if err != nil {
		return nil, err
	}
	gdb, _ := db.DB()
	gdb.SetConnMaxIdleTime(5 * time.Minute)
	gdb.SetConnMaxLifetime(1 * time.Hour)
	gdb.SetMaxIdleConns(20)
	gdb.SetMaxOpenConns(100)
	if err := gdb.Ping(); err != nil {
		return nil, err
	}
	return db, err
}

type log struct {
	logger *zap.Logger
	level  logger.LogLevel
}

func (z *log) LogMode(level logger.LogLevel) logger.Interface {
	return &log{level: level, logger: z.logger}
}

func (z *log) Info(ctx context.Context, msg string, data ...any) {
	if z.logger == nil {
		return
	}
	z.logger.Info("SQL执行Info", zap.String("msg", msg), zap.Any("data", data))
}

func (z *log) Warn(ctx context.Context, msg string, data ...any) {
	if z.logger == nil {
		return
	}
	z.logger.Warn("SQL执行Warn", zap.String("msg", msg), zap.Any("data", data))
}

func (z *log) Error(ctx context.Context, msg string, data ...any) {
	if z.logger == nil {
		return
	}
	z.logger.Error("SQL执行Error", zap.String("msg", msg), zap.Any("data", data))
}

func (z *log) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if z.logger == nil {
		return
	}
	var sql string
	var rows int64
	var sqlLoaded bool = false
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}
		var mysqlErr *mysqldriver.MySQLError
		if errors.As(err, &mysqlErr) {
			if mysqlErr.Number == 1062 {
				return
			}
		}
		sql, rows = fc()
		sqlLoaded = true
		z.logger.Error("SQL执行失败", zap.String("sql", sql), zap.Int64("rows", rows), zap.Error(err))
	}
	if z.level >= logger.Info {
		elapsed := time.Since(begin)
		if !sqlLoaded {
			sql, rows = fc()
		}
		z.logger.Info("SQL执行日志", zap.String("sql", sql), zap.Duration("elapsed", elapsed))
	}
}
