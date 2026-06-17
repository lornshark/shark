// Package sharkmongodb 提供了 MongoDB 客户端的创建和连接管理。
//
// 封装了 MongoDB Go Driver v2 的连接创建和健康检查。
package sharkmongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Config 是 MongoDB 的连接配置。
//
// 支持从 JSON、YAML、viper（mapstructure）等多种配置源加载。
type Config struct {
	// Host MongoDB 连接地址，格式为 "host:port"（如 "127.0.0.1:27017"）
	Host string `json:"host" yaml:"host" mapstructure:"host"`
	// User 数据库用户名，为空时使用匿名访问
	User string `json:"user" yaml:"user" mapstructure:"user"`
	// Password 数据库密码，为空时使用匿名访问
	Password string `json:"password" yaml:"password" mapstructure:"password"`
}

// New 创建 MongoDB 客户端并验证连接。
//
// 连接 URI 格式: mongodb://user:password@host/?authSource=admin
// 认证源固定为 admin 数据库。
//
// 使用示例:
//
//	client, err := sharkmongodb.New(ctx, &sharkmongodb.Config{
//	    Host:     "127.0.0.1:27017",
//	    User:     "admin",
//	    Password: "password",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Disconnect(ctx)
//
//	// 使用客户端操作数据库
//	coll := client.Database("mydb").Collection("users")
//	coll.InsertOne(ctx, bson.D{{Key: "name", Value: "张三"}})
//
// 参数:
//   - ctx: 上下文，用于连接超时和健康检查
//   - config: MongoDB 连接配置，不能为 nil
//
// 返回值:
//   - *mongo.Client: 已通过 Ping 验证的 MongoDB 客户端
//   - error: 配置为空、连接失败或 Ping 不通时返回错误
func New(ctx context.Context, config *Config) (*mongo.Client, error) {
	if config == nil {
		return nil, fmt.Errorf("config required")
	}

	// 构建 MongoDB 连接 URI
	uri := fmt.Sprintf("mongodb://%v:%v@%v/?authSource=admin", config.User, config.Password, config.Host)
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	// Ping 验证连接可用性
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}
	return client, nil
}
