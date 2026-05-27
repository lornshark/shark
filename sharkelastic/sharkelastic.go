package sharkelastic

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/olivere/elastic/v7"
)

type Config struct {
	Host     string `json:"host"`     // 连接地址
	User     string `json:"user"`     // 连接用户名
	Password string `json:"password"` // 连接密码
}

func New(config *Config) (*elastic.Client, error) {
	if config == nil {
		return nil, fmt.Errorf("config required")
	}
	if config.Host == "" {
		config.Host = "localhost:9200"
	}
	httpClient := http.Client{}
	httpClient.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	es, err := elastic.NewClient(
		elastic.SetHttpClient(&httpClient),
		elastic.SetSniff(false),
		elastic.SetHealthcheck(true),
		elastic.SetHealthcheckInterval(15*time.Second),
		elastic.SetURL(config.Host),
		elastic.SetBasicAuth(config.User, config.Password),
		elastic.SetSniff(false),
		elastic.SetHealthcheck(false),
	)
	if err != nil {
		return nil, err
	}
	_, err = es.NodesInfo().Do(context.Background())
	if err != nil {
		return nil, err
	}
	return es, nil
}
