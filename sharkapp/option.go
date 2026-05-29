package sharkapp

import (
	"context"
	"errors"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/lornshark/shark/sharkdb"
	"github.com/lornshark/shark/sharkelastic"
	"github.com/lornshark/shark/sharkkafka"
	"github.com/lornshark/shark/sharkminio"
	"github.com/lornshark/shark/sharkmongodb"
	"github.com/lornshark/shark/sharkrabbitmq"
	"github.com/lornshark/shark/sharkredis"
	"github.com/lornshark/shark/sharkrisingwave"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cast"
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

func NewOption() *Options {
	return &Options{}
}

func NewOptionWithRedis(project, name, id, env, host, port, password string) *Options {
	options := &Options{
		project: project,
		name:    name,
		id:      id,
		env:     env,
	}
	r, err := sharkredis.New(context.Background(), &sharkredis.Config{
		Host:     host,
		Port:     cast.ToInt(port),
		Password: password,
	})
	if err != nil {
		panic(err)
	}
	value, err := r.Get(context.Background(), fmt.Sprintf("%v:system:config:%v-%v", project, name, id)).Result()
	// 配置不存在的时候,初始化一下配置,默认都是关闭的
	if err != nil && errors.Is(err, redis.Nil) {
		v := map[string]any{
			"redis":      false,
			"db":         false,
			"elastic":    false,
			"kafka":      false,
			"minio":      false,
			"mongodb":    false,
			"rabbitmq":   false,
			"risingwave": false,
			"timer":      false,
			"pprof":      0,
			"health":     0,
			"grpc":       0,
			"http":       0,
		}
		bytes, err := sonic.Marshal(v)
		if err != nil {
			panic(err)
		}
		err = r.Set(context.Background(), fmt.Sprintf("%v:system:config:%v-%v", project, name, id), string(bytes), 0).Err()
		if err != nil {
			panic(err)
		}
		return options
	}
	rc := map[string]any{}
	err = sonic.Unmarshal([]byte(value), &rc)
	if err != nil {
		panic(err)
	}
	if v, ok := rc["redis"]; ok && cast.ToBool(v) {
		value := r.Get(context.Background(), project+":system:config:redis").Val()
		config := &sharkredis.Config{}
		err = sonic.Unmarshal([]byte(value), config)
		if err != nil {
			panic(err)
		}
		options = options.WithRedis(config)
	}
	if v, ok := rc["db"]; ok && cast.ToBool(v) {
		value := r.Get(context.Background(), project+":system:config:db").Val()
		config := &sharkdb.Config{}
		err = sonic.Unmarshal([]byte(value), config)
		if err != nil {
			panic(err)
		}
		options = options.WithDb(config)
	}
	if v, ok := rc["elastic"]; ok && cast.ToBool(v) {
		value := r.Get(context.Background(), project+":system:config:elastic").Val()
		config := &sharkelastic.Config{}
		err = sonic.Unmarshal([]byte(value), config)
		if err != nil {
			panic(err)
		}
		options = options.WithElastic(config)
	}
	if v, ok := rc["kafka"]; ok && cast.ToBool(v) {
		value := r.Get(context.Background(), project+":system:config:kafka").Val()
		config := &sharkkafka.Config{}
		err = sonic.Unmarshal([]byte(value), config)
		if err != nil {
			panic(err)
		}
		options = options.WithKafka(config)
	}
	if v, ok := rc["minio"]; ok && cast.ToBool(v) {
		value := r.Get(context.Background(), project+":system:config:minio").Val()
		config := &sharkminio.Config{}
		err = sonic.Unmarshal([]byte(value), config)
		if err != nil {
			panic(err)
		}
		options = options.WithMinio(config)
	}
	if v, ok := rc["mongodb"]; ok && cast.ToBool(v) {
		value := r.Get(context.Background(), project+":system:config:mongodb").Val()
		config := &sharkmongodb.Config{}
		err = sonic.Unmarshal([]byte(value), config)
		if err != nil {
			panic(err)
		}
		options = options.WithMongodb(config)
	}
	if v, ok := rc["rabbitmq"]; ok && cast.ToBool(v) {
		value := r.Get(context.Background(), project+":system:config:rabbitmq").Val()
		config := &sharkrabbitmq.Config{}
		err = sonic.Unmarshal([]byte(value), config)
		if err != nil {
			panic(err)
		}
		options = options.WithRabbitmq(config)
	}
	if v, ok := rc["risingwave"]; ok && cast.ToBool(v) {
		value := r.Get(context.Background(), project+":system:config:risingwave").Val()
		config := &sharkrisingwave.Config{}
		err = sonic.Unmarshal([]byte(value), config)
		if err != nil {
			panic(err)
		}
		options = options.WithRisingwave(config)
	}
	if v, ok := rc["timer"]; ok && cast.ToBool(v) {
		options = options.WithTimer(true)
	}
	if v, ok := rc["pprof"]; ok && cast.ToInt(v) > 0 {
		options = options.WithPprof(cast.ToInt(v))
	}
	if v, ok := rc["health"]; ok && cast.ToInt(v) > 0 {
		options = options.WithHealth(cast.ToInt(v))
	}
	if v, ok := rc["grpc"]; ok && cast.ToInt(v) > 0 {
		options = options.WithGrpc(cast.ToInt(v))
	}
	if v, ok := rc["http"]; ok && cast.ToInt(v) > 0 {
		options = options.WithHttp(cast.ToInt(v))
	}
	return options
}

func (s *Options) WithProject(project string) *Options {
	s.project = project
	return s
}

func (s *Options) WithName(name string) *Options {
	s.name = name
	return s
}

func (s *Options) WithId(id string) *Options {
	s.id = id
	return s
}

func (s *Options) WithEnv(env string) *Options {
	s.env = env
	return s
}

func (s *Options) WithKafka(kafka *sharkkafka.Config) *Options {
	s.kafka = kafka
	return s
}

func (s *Options) WithRedis(redis *sharkredis.Config) *Options {
	s.redis = redis
	return s
}

func (s *Options) WithDb(config *sharkdb.Config) *Options {
	s.db = config
	return s
}

func (s *Options) WithElastic(elastic *sharkelastic.Config) *Options {
	s.elastic = elastic
	return s
}

func (s *Options) WithRabbitmq(rabbitmq *sharkrabbitmq.Config) *Options {
	s.rabbitmq = rabbitmq
	return s
}

func (s *Options) WithRisingwave(risingwave *sharkrisingwave.Config) *Options {
	s.risingwave = risingwave
	return s
}

func (s *Options) WithMongodb(mongodb *sharkmongodb.Config) *Options {
	s.mongodb = mongodb
	return s
}

func (s *Options) WithMinio(minio *sharkminio.Config) *Options {
	s.minio = minio
	return s
}

func (s *Options) WithTimer(timer bool) *Options {
	s.timer = timer
	return s
}

func (s *Options) WithPprof(port int) *Options {
	s.pprof = port
	return s
}

func (s *Options) WithHealth(port int) *Options {
	s.health = port
	return s
}

func (s *Options) WithGrpc(port int) *Options {
	s.grpc = port
	return s
}

func (s *Options) WithHttp(port int) *Options {
	s.http = port
	return s
}
