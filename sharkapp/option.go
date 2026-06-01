package sharkapp

import (
	"os"
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
	id         string
	db         *sharkdb.Config
	env        string
	name       string
	minio      *sharkminio.Config
	timer      bool
	pprof      int
	kafka      *sharkkafka.Config
	redis      *sharkredis.Config
	elastic    *sharkelastic.Config
	project    string
	mongodb    *sharkmongodb.Config
	rabbitmq   *sharkrabbitmq.Config
	grpc       int
	health     int
	risingwave *sharkrisingwave.Config
	http       int
}

func NewOption(project string, name string) *Options {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}
	env := "dev"
	id := "1"
	// 如果在 k8s 环境中，优先使用环境变量中的配置
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		viper.AutomaticEnv()
		env = viper.GetString("env")
		id = viper.GetString("id")
	}
	options := &Options{
		project: project,
		name:    name,
		env:     env,
		id:      id,
	}
	viper.UnmarshalKey("redis", &options.redis)
	viper.UnmarshalKey("db", &options.db)
	viper.UnmarshalKey("minio", &options.minio)
	viper.UnmarshalKey("kafka", &options.kafka)
	viper.UnmarshalKey("elastic", &options.elastic)
	viper.UnmarshalKey("mongodb", &options.mongodb)
	viper.UnmarshalKey("rabbitmq", &options.rabbitmq)
	viper.UnmarshalKey("risingwave", &options.risingwave)
	viper.UnmarshalKey("timer", &options.timer)
	viper.UnmarshalKey("pprof", &options.pprof)
	viper.UnmarshalKey("grpc", &options.grpc)
	viper.UnmarshalKey("health", &options.health)
	viper.UnmarshalKey("http", &options.http)
	return options
}
