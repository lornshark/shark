package sharkrisingwave

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Config struct {
	Host     string `json:"host" yaml:"host" mapstructure:"host"`             // 连接地址
	Port     int    `json:"port" yaml:"port" mapstructure:"port"`             // 连接端口
	User     string `json:"user" yaml:"user" mapstructure:"user"`             // 连接用户名
	Password string `json:"password" yaml:"password" mapstructure:"password"` // 连接密码
	Database string `json:"database" yaml:"database" mapstructure:"database"` // 连接数据库名称
}

func New(ctx context.Context, logger *zap.Logger, config *Config) (*gorm.DB, error) {
	if config == nil {
		return nil, fmt.Errorf("config required")
	}
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		url.QueryEscape(config.User),
		url.QueryEscape(config.Password),
		config.Host,
		config.Port,
		config.Database,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: &log{
			logger: logger,
		},
	})
	if err != nil {
		return nil, err
	}
	gdb, _ := db.DB()
	gdb.SetConnMaxIdleTime(10 * time.Minute)
	gdb.SetConnMaxLifetime(1 * time.Hour)
	gdb.SetMaxIdleConns(10)
	gdb.SetMaxOpenConns(30)
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
	z.logger.Info("RW执行Info", zap.String("msg", msg), zap.Any("data", data))
}

func (z *log) Warn(ctx context.Context, msg string, data ...any) {
	if z.logger == nil {
		return
	}
	z.logger.Warn("RW执行Warn", zap.String("msg", msg), zap.Any("data", data))
}

func (z *log) Error(ctx context.Context, msg string, data ...any) {
	if z.logger == nil {
		return
	}
	z.logger.Error("RW执行Error", zap.String("msg", msg), zap.Any("data", data))
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" {
				return
			}
		}
		sql, rows = fc()
		sqlLoaded = true
		z.logger.Error("RW执行失败", zap.String("sql", sql), zap.Int64("rows", rows), zap.Error(err))
	}
	if z.level >= logger.Info {
		elapsed := time.Since(begin)
		if !sqlLoaded {
			sql, rows = fc()
		}
		z.logger.Info("RW执行日志", zap.String("sql", sql), zap.Int64("rows", rows), zap.Duration("elapsed", elapsed))
	}
}
