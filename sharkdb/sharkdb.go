// Package sharkdb 提供了 MySQL 数据库的连接管理和 GORM 日志适配功能。
//
// 本包基于 GORM（gorm.io/gorm）和 go-sql-driver/mysql 驱动，
// 封装了数据库连接的创建、连接池配置以及将 GORM 日志输出对接 zap 日志系统。
//
// 特性:
//   - 支持 TLS 加密连接（如 TiDB、云数据库）
//   - 预编译语句缓存（PrepareStmt）提升查询性能
//   - 跳过默认事务（提升读操作性能）
//   - 智能日志过滤（忽略 RecordNotFound 和 DuplicateKey 错误）
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

// Config 是 MySQL 数据库的连接配置。
//
// 支持从 JSON、YAML、viper（mapstructure）等多种配置源加载。
type Config struct {
	// Host 数据库连接地址，格式为 "host:port"，如 "127.0.0.1:3306"
	Host string `json:"host" yaml:"host" mapstructure:"host"`
	// User 数据库用户名
	User string `json:"user" yaml:"user" mapstructure:"user"`
	// Password 数据库密码
	Password string `json:"password" yaml:"password" mapstructure:"password"`
	// Database 要连接的数据库名称
	Database string `json:"database" yaml:"database" mapstructure:"database"`
	// Tls TLS 配置名称，非空时启用 TLS 加密连接
	// 默认为空字符串表示不使用 TLS，设置为 "tidb" 或其他值会注册对应的 TLS 配置
	Tls string `json:"tls" yaml:"tls" mapstructure:"tls"`
}

// NewDb 创建并配置一个 GORM 数据库连接。
//
// 连接配置:
//   - 字符集: utf8mb4（支持完整 Unicode，包括 emoji）
//   - 时区: Local（使用服务器本地时区）
//   - parseTime: true（自动将 MySQL 时间类型转为 Go time.Time）
//
// GORM 配置:
//   - SkipDefaultTransaction: 跳过默认事务，避免每次写操作自动开启事务，提升性能
//   - PrepareStmt: 启用预编译语句缓存，重复查询更快
//   - CreateBatchSize: 批量插入时每批最多 1000 条
//   - DisableForeignKeyConstraintWhenMigrating: 自动迁移时不创建外键约束
//
// 连接池配置:
//   - ConnMaxIdleTime: 空闲连接最大存活时间 5 分钟
//   - ConnMaxLifetime: 连接最大存活时间 1 小时
//   - MaxIdleConns: 最大空闲连接数 20
//   - MaxOpenConns: 最大打开连接数 100
//
// 使用示例:
//
//	db, err := sharkdb.NewDb(ctx, logger, &sharkdb.Config{
//	    Host:     "127.0.0.1:3306",
//	    User:     "root",
//	    Password: "password",
//	    Database: "mydb",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// 使用 GORM 操作数据库
//	db.Table("users").Where("age > ?", 18).Find(&users)
//	db.Create(&User{Name: "张三", Age: 25})
//
// 参数:
//   - ctx: 上下文（当前未使用，保留用于未来扩展）
//   - logger: zap 日志记录器，用于输出 SQL 执行日志
//   - config: 数据库连接配置，不能为 nil
//
// 返回值:
//   - *gorm.DB: 已配置并验证通过的数据库连接
//   - error: 配置为空、连接失败或 Ping 不通时返回错误
func NewDb(ctx context.Context, logger *zap.Logger, config *Config) (*gorm.DB, error) {
	if config == nil {
		return nil, fmt.Errorf("config required")
	}

	// 构建 MySQL DSN（Data Source Name）
	// 格式：user:password@tcp(host)/database?charset=utf8mb4&parseTime=True&loc=Local
	dsn := "%v:%v@tcp(%v)/%v?charset=utf8mb4&parseTime=True&loc=Local"
	dsn = fmt.Sprintf(dsn, config.User, config.Password, config.Host, config.Database)

	// 如果配置了 TLS，注册 TLS 配置并附加到 DSN
	if config.Tls != "" {
		dsn += "&tls=tidb"
		// 注册名为 "tidb" 的 TLS 配置，最低要求 TLS 1.2
		mysqldriver.RegisterTLSConfig("tidb", &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: config.Host,
		})
	}

	// 使用 GORM 打开数据库连接
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		SkipDefaultTransaction:                   true,                 // 跳过默认事务
		PrepareStmt:                              true,                 // 启用预编译语句缓存
		CreateBatchSize:                          1000,                 // 批量创建每批 1000 条
		DisableForeignKeyConstraintWhenMigrating: true,                 // 迁移时不创建外键
		Logger:                                   &log{logger: logger}, // 自定义日志适配器
	})
	if err != nil {
		return nil, err
	}

	// 获取底层 *sql.DB 实例以配置连接池
	gdb, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB failed: %w", err)
	}
	gdb.SetConnMaxIdleTime(5 * time.Minute) // 空闲连接 5 分钟后关闭
	gdb.SetConnMaxLifetime(1 * time.Hour)   // 连接最长存活 1 小时
	gdb.SetMaxIdleConns(20)                 // 最多保持 20 个空闲连接
	gdb.SetMaxOpenConns(100)                // 最多打开 100 个连接

	// Ping 验证连接是否可用
	if err := gdb.Ping(); err != nil {
		return nil, err
	}
	return db, err
}

// log 是 GORM logger.Interface 的自定义实现，
// 将 GORM 的 SQL 日志输出对接到 zap 结构化日志系统。
//
// 它实现了日志级别过滤和智能错误过滤：
//   - 忽略 ErrRecordNotFound（查询无结果，不是异常）
//   - 忽略 MySQL 错误码 1062（Duplicate entry，主键/唯一键冲突，业务常见）
type log struct {
	logger *zap.Logger     // zap 日志记录器
	level  logger.LogLevel // GORM 日志级别
}

// LogMode 设置日志级别，返回新的 logger 实例。
// GORM 会在不同场景下调用此方法来调整日志输出级别。
func (z *log) LogMode(level logger.LogLevel) logger.Interface {
	return &log{level: level, logger: z.logger}
}

// Info 输出 Info 级别的 SQL 日志。
// 对应 GORM 的普通信息日志（如连接信息、表迁移等）。
func (z *log) Info(ctx context.Context, msg string, data ...any) {
	if z.logger == nil {
		return
	}
	z.logger.Info("SQL执行Info", zap.String("msg", msg), zap.Any("data", data))
}

// Warn 输出 Warn 级别的 SQL 日志。
// 对应 GORM 的警告信息（如慢查询警告等）。
func (z *log) Warn(ctx context.Context, msg string, data ...any) {
	if z.logger == nil {
		return
	}
	z.logger.Warn("SQL执行Warn", zap.String("msg", msg), zap.Any("data", data))
}

// Error 输出 Error 级别的 SQL 日志。
// 对应 GORM 的错误信息（不在 Trace 中处理的错误）。
func (z *log) Error(ctx context.Context, msg string, data ...any) {
	if z.logger == nil {
		return
	}
	z.logger.Error("SQL执行Error", zap.String("msg", msg), zap.Any("data", data))
}

// Trace 是 GORM 最核心的日志回调，每次 SQL 执行后都会调用。
//
// 通过 begin 和 fc（延迟获取 SQL 的函数）记录了：
//   - 执行的 SQL 语句
//   - 影响的行数
//   - 执行耗时
//
// 智能过滤策略:
//   - 忽略 gorm.ErrRecordNotFound：查询无结果是正常业务逻辑
//   - 忽略 MySQL 1062 Duplicate entry：主键/唯一键冲突是业务常见场景
//   - 其他错误以 Error 级别记录 SQL 和错误信息
//   - 正常 SQL 在 Info 级别记录 SQL 和耗时
//
// 参数:
//   - ctx: 上下文
//   - begin: SQL 开始执行的时间
//   - fc: 延迟获取 SQL 语句和影响行数的函数（仅在需要时调用，避免不必要的字符串拼接）
//   - err: SQL 执行的错误结果
func (z *log) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if z.logger == nil {
		return
	}

	var sql string
	var rows int64
	var sqlLoaded bool = false

	// 错误处理：先判断是否需要过滤
	if err != nil {
		// 忽略 "record not found" 错误（查询无结果，非异常）
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}
		// 忽略 MySQL 错误码 1062（Duplicate entry，重复键冲突）
		var mysqlErr *mysqldriver.MySQLError
		if errors.As(err, &mysqlErr) {
			if mysqlErr.Number == 1062 {
				return
			}
		}
		// 其他错误：获取 SQL 并记录
		sql, rows = fc()
		sqlLoaded = true
		z.logger.Error("SQL执行失败", zap.String("sql", sql), zap.Int64("rows", rows), zap.Error(err))
	}

	// 正常日志：Info 级别
	if z.level >= logger.Info {
		elapsed := time.Since(begin)
		// 如果上面已经获取过 SQL，则不需要重复获取
		if !sqlLoaded {
			sql, rows = fc()
		}
		z.logger.Info("SQL执行日志", zap.String("sql", sql), zap.Duration("elapsed", elapsed))
	}
}
