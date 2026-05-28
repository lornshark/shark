package sharkminio

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	Host     string `json:"host" yaml:"host" mapstructure:"host"`             // 连接地
	Port     int    `json:"port" yaml:"port" mapstructure:"port"`             // 连接端口
	User     string `json:"user" yaml:"user" mapstructure:"user"`             // 连接用户名，默认值为 "" 不起用验证
	Password string `json:"password" yaml:"password" mapstructure:"password"` // 连接密码，默认值为 "" 不起用验证
}

func New(ctx context.Context, config *Config) (*minio.Client, error) {
	if config == nil {
		return nil, fmt.Errorf("config required")
	}
	endpoint := fmt.Sprintf("%s:%d", config.Host, config.Port)
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.User, config.Password, ""),
		Secure: false,
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}
