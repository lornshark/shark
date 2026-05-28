package sharkmongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Config struct {
	Host     string `json:"host"`     // 连接地
	Port     int    `json:"port"`     // 连接端口
	User     string `json:"user"`     // 连接用户名，默认值为 "" 不起用验证
	Password string `json:"password"` // 连接密码，默认值为 "" 不起用验证
}

func New(ctx context.Context, config *Config) (*mongo.Client, error) {
	if config == nil {
		return nil, fmt.Errorf("config required")
	}
	uri := fmt.Sprintf("mongodb://%v:%v@%v:%v/?authSource=admin", config.User, config.Password, config.Host, config.Port)
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}
	return client, nil
}
