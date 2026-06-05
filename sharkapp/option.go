package sharkapp

import (
	"strings"

	"github.com/lornshark/shark/sharkdb"
	"github.com/lornshark/shark/sharkelastic"
	"github.com/lornshark/shark/sharkkafka"
	"github.com/lornshark/shark/sharkminio"
	"github.com/lornshark/shark/sharkmongodb"
	"github.com/lornshark/shark/sharkrabbitmq"
	"github.com/lornshark/shark/sharkredis"
	"github.com/lornshark/shark/sharkrisingwave"
	"github.com/spf13/viper"
)

type Options struct {
	id            string
	db            *sharkdb.Config
	env           string
	name          string
	minio         *sharkminio.Config
	timer         bool
	pprof         int
	kafka         *sharkkafka.Config
	redis         *sharkredis.Config
	redis_cluster *sharkredis.Config
	redis_client  *sharkredis.Config
	elastic       *sharkelastic.Config
	project       string
	mongodb       *sharkmongodb.Config
	rabbitmq      *sharkrabbitmq.Config
	grpc          int
	health        int
	risingwave    *sharkrisingwave.Config
	http          int
}

func NewOption(project string, name string) *Options {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	v.SetDefault("env", "dev")
	v.SetDefault("id", "1")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			panic(err)
		}
	}
	env := strings.TrimSpace(v.GetString("env"))
	id := strings.TrimSpace(v.GetString("id"))
	options := &Options{
		project: project,
		name:    name,
		env:     env,
		id:      id,
	}
	options.timer = v.GetBool("timer")
	options.pprof = v.GetInt("pprof")
	options.grpc = v.GetInt("grpc")
	options.health = v.GetInt("health")
	options.http = v.GetInt("http")
	if strings.TrimSpace(v.GetString("redis_cluster.host")) != "" {
		options.redis_cluster = &sharkredis.Config{
			Host:     strings.TrimSpace(v.GetString("redis_cluster.host")),
			Port:     v.GetInt("redis_cluster.port"),
			Password: strings.TrimSpace(v.GetString("redis_cluster.password")),
		}
	}
	if strings.TrimSpace(v.GetString("redis_client.host")) != "" {
		options.redis_client = &sharkredis.Config{
			Host:     strings.TrimSpace(v.GetString("redis_client.host")),
			Port:     v.GetInt("redis_client.port"),
			Password: strings.TrimSpace(v.GetString("redis_client.password")),
		}
	}
	if strings.TrimSpace(v.GetString("redis.host")) != "" {
		options.redis = &sharkredis.Config{
			Host:     strings.TrimSpace(v.GetString("redis.host")),
			Port:     v.GetInt("redis.port"),
			Password: strings.TrimSpace(v.GetString("redis.password")),
		}
	}
	if strings.TrimSpace(v.GetString("db.host")) != "" {
		options.db = &sharkdb.Config{
			Host:     strings.TrimSpace(v.GetString("db.host")),
			Port:     v.GetInt("db.port"),
			User:     strings.TrimSpace(v.GetString("db.user")),
			Password: strings.TrimSpace(v.GetString("db.password")),
			Database: strings.TrimSpace(v.GetString("db.database")),
		}
	}
	if strings.TrimSpace(v.GetString("elastic.host")) != "" {
		options.elastic = &sharkelastic.Config{
			Host:     strings.TrimSpace(v.GetString("elastic.host")),
			User:     strings.TrimSpace(v.GetString("elastic.user")),
			Password: strings.TrimSpace(v.GetString("elastic.password")),
		}
	}
	if v.GetString("minio.host") != "" {
		options.minio = &sharkminio.Config{
			Host:     strings.TrimSpace(v.GetString("minio.host")),
			Port:     v.GetInt("minio.port"),
			User:     strings.TrimSpace(v.GetString("minio.user")),
			Password: strings.TrimSpace(v.GetString("minio.password")),
		}
	}
	if v.GetString("kafka.host") != "" {
		options.kafka = &sharkkafka.Config{
			Host:     strings.TrimSpace(v.GetString("kafka.host")),
			Port:     v.GetInt("kafka.port"),
			User:     strings.TrimSpace(v.GetString("kafka.user")),
			Password: strings.TrimSpace(v.GetString("kafka.password")),
		}
	}
	if v.GetString("mongodb.host") != "" {
		options.mongodb = &sharkmongodb.Config{
			Host:     strings.TrimSpace(v.GetString("mongodb.host")),
			Port:     v.GetInt("mongodb.port"),
			User:     strings.TrimSpace(v.GetString("mongodb.user")),
			Password: strings.TrimSpace(v.GetString("mongodb.password")),
		}
	}
	rmqhosts := v.GetStringSlice("rabbitmq.host")
	if len(rmqhosts) == 0 {
		if s := strings.TrimSpace(v.GetString("rabbitmq.host")); s != "" {
			sp := strings.Split(s, ",")
			for _, h := range sp {
				if strings.TrimSpace(h) != "" {
					rmqhosts = append(rmqhosts, strings.TrimSpace(h))
				}
			}
		}
	}
	if len(rmqhosts) > 0 {
		newHosts := []string{}
		for i := range rmqhosts {
			if strings.Contains(rmqhosts[i], ",") {
				sp := strings.Split(rmqhosts[i], ",")
				for _, h := range sp {
					if strings.TrimSpace(h) != "" {
						newHosts = append(newHosts, strings.TrimSpace(h))
					}
				}
			} else {
				newHosts = append(newHosts, rmqhosts[i])
			}
		}
		rmqhosts = newHosts
		options.rabbitmq = &sharkrabbitmq.Config{
			Host:     rmqhosts,
			User:     strings.TrimSpace(v.GetString("rabbitmq.user")),
			Password: strings.TrimSpace(v.GetString("rabbitmq.password")),
		}
	}
	if v.GetString("risingwave.host") != "" {
		options.risingwave = &sharkrisingwave.Config{
			Host:     strings.TrimSpace(v.GetString("risingwave.host")),
			Port:     v.GetInt("risingwave.port"),
			User:     strings.TrimSpace(v.GetString("risingwave.user")),
			Password: strings.TrimSpace(v.GetString("risingwave.password")),
			Database: strings.TrimSpace(v.GetString("risingwave.database")),
		}
	}
	return options
}
