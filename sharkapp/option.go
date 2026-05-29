package sharkapp

import (
	"context"
	"fmt"

	"github.com/lornshark/shark/sharkdb"
	"github.com/lornshark/shark/sharkelastic"
	"github.com/lornshark/shark/sharkjson"
	"github.com/lornshark/shark/sharkkafka"
	"github.com/lornshark/shark/sharkminio"
	"github.com/lornshark/shark/sharkmongodb"
	"github.com/lornshark/shark/sharkrabbitmq"
	"github.com/lornshark/shark/sharkredis"
	"github.com/lornshark/shark/sharkrisingwave"
	"github.com/spf13/cast"
)

type Options struct {
	config_redis_host     string
	config_redis_port     string
	config_redis_password string

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

func NewOption(env, project, name, id string) *Options {
	return &Options{
		project: project,
		name:    name,
		id:      id,
		env:     env,
	}
}

func (s *Options) WithKafka(config *sharkkafka.Config) *Options {
	if config == nil {
		r, err := sharkredis.New(context.Background(), &sharkredis.Config{
			Host:     s.config_redis_host,
			Port:     cast.ToInt(s.config_redis_port),
			Password: s.config_redis_password,
		})
		if err != nil {
			return s
		}
		value, err := r.Get(context.Background(), s.project+":system:config:kafka").Result()
		if err != nil {
			return s
		}
		config = sharkjson.ParseJsonString[sharkkafka.Config](value)
	}
	s.kafka = config
	return s
}

func (s *Options) WithRedis(config *sharkredis.Config) *Options {
	if config == nil {
		r, err := sharkredis.New(context.Background(), &sharkredis.Config{
			Host:     s.config_redis_host,
			Port:     cast.ToInt(s.config_redis_port),
			Password: s.config_redis_password,
		})
		if err != nil {
			return s
		}
		value, err := r.Get(context.Background(), s.project+":system:config:redis").Result()
		if err != nil {
			return s
		}
		config = sharkjson.ParseJsonString[sharkredis.Config](value)
	}
	s.redis = config
	return s
}

func (s *Options) WithDb(config *sharkdb.Config) *Options {
	if config == nil {
		r, err := sharkredis.New(context.Background(), &sharkredis.Config{
			Host:     s.config_redis_host,
			Port:     cast.ToInt(s.config_redis_port),
			Password: s.config_redis_password,
		})
		if err != nil {
			return s
		}
		value, err := r.Get(context.Background(), s.project+":system:config:db").Result()
		if err != nil {
			return s
		}
		config = sharkjson.ParseJsonString[sharkdb.Config](value)
	}
	s.db = config
	return s
}

func (s *Options) WithElastic(config *sharkelastic.Config) *Options {
	if config == nil {
		r, err := sharkredis.New(context.Background(), &sharkredis.Config{
			Host:     s.config_redis_host,
			Port:     cast.ToInt(s.config_redis_port),
			Password: s.config_redis_password,
		})
		if err != nil {
			return s
		}
		value, err := r.Get(context.Background(), s.project+":system:config:elastic").Result()
		if err != nil {
			return s
		}
		config = sharkjson.ParseJsonString[sharkelastic.Config](value)
	}
	s.elastic = config
	return s
}

func (s *Options) WithRabbitmq(config *sharkrabbitmq.Config) *Options {
	if config == nil {
		r, err := sharkredis.New(context.Background(), &sharkredis.Config{
			Host:     s.config_redis_host,
			Port:     cast.ToInt(s.config_redis_port),
			Password: s.config_redis_password,
		})
		if err != nil {
			return s
		}
		value, err := r.Get(context.Background(), s.project+":system:config:rabbitmq").Result()
		if err != nil {
			return s
		}
		config = sharkjson.ParseJsonString[sharkrabbitmq.Config](value)
	}
	s.rabbitmq = config
	return s
}

func (s *Options) WithRisingwave(config *sharkrisingwave.Config) *Options {
	if config == nil {
		r, err := sharkredis.New(context.Background(), &sharkredis.Config{
			Host:     s.config_redis_host,
			Port:     cast.ToInt(s.config_redis_port),
			Password: s.config_redis_password,
		})
		if err != nil {
			return s
		}
		value, err := r.Get(context.Background(), s.project+":system:config:risingwave").Result()
		if err != nil {
			return s
		}
		config = sharkjson.ParseJsonString[sharkrisingwave.Config](value)
	}
	s.risingwave = config
	return s
}

func (s *Options) WithMongodb(config *sharkmongodb.Config) *Options {
	if config == nil {
		r, err := sharkredis.New(context.Background(), &sharkredis.Config{
			Host:     s.config_redis_host,
			Port:     cast.ToInt(s.config_redis_port),
			Password: s.config_redis_password,
		})
		if err != nil {
			return s
		}
		value, err := r.Get(context.Background(), s.project+":system:config:mongodb").Result()
		if err != nil {
			return s
		}
		config = sharkjson.ParseJsonString[sharkmongodb.Config](value)
	}
	s.mongodb = config
	return s
}

func (s *Options) WithMinio(config *sharkminio.Config) *Options {
	if config == nil {
		r, err := sharkredis.New(context.Background(), &sharkredis.Config{
			Host:     s.config_redis_host,
			Port:     cast.ToInt(s.config_redis_port),
			Password: s.config_redis_password,
		})
		if err != nil {
			return s
		}
		value, err := r.Get(context.Background(), s.project+":system:config:minio").Result()
		if err != nil {
			return s
		}
		config = sharkjson.ParseJsonString[sharkminio.Config](value)
	}

	s.minio = config
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

func NewOptionWithRedis(env, project, name, id, host, port, password string) *Options {
	options := NewOption(env, project, name, id)
	options.config_redis_host = host
	options.config_redis_port = port
	options.config_redis_password = password
	k := fmt.Sprintf("%v:system:config:%v-%v", project, name, id)
	r, err := sharkredis.New(context.Background(), &sharkredis.Config{
		Host:     host,
		Port:     cast.ToInt(port),
		Password: password,
	})
	if err != nil {
		return options
	}
	value, err := r.Get(context.Background(), k).Result()
	if err != nil {
		return options
	}
	mvalue := sharkjson.ParseJsonString[map[string]any](value)
	if mvalue != nil {
		options.WithTimer(cast.ToBool((*mvalue)["timer"]))
		options.WithHttp(cast.ToInt((*mvalue)["http"]))
		options.WithGrpc(cast.ToInt((*mvalue)["grpc"]))
		options.WithPprof(cast.ToInt((*mvalue)["pprof"]))
		options.WithHealth(cast.ToInt((*mvalue)["health"]))
	}
	return options
}
