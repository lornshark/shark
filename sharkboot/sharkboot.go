package sharkboot

import (
	"context"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/lornshark/shark/sharkapp"
	"github.com/lornshark/shark/sharkdb"
	"github.com/lornshark/shark/sharkelastic"
	"github.com/lornshark/shark/sharkkafka"
	"github.com/lornshark/shark/sharkminio"
	"github.com/lornshark/shark/sharkmongodb"
	"github.com/lornshark/shark/sharkrabbitmq"
	"github.com/lornshark/shark/sharkredis"
	"github.com/lornshark/shark/sharkrisingwave"
	"github.com/spf13/cast"
)

type Boot struct {
	redis_host     string
	redis_port     string
	redis_password string
	project        string
	name           string
	id             string
	env            string
	redis          bool
	db             bool
	elastic        bool
	kafka          bool
	minio          bool
	mongodb        bool
	rabbitmq       bool
	risingwave     bool
	timer          bool
}

func New(project, name, id, env, host, port, password string) *Boot {
	return &Boot{
		redis_host:     host,
		redis_port:     port,
		redis_password: password,
		project:        project,
		name:           name,
		id:             id,
		env:            env,
	}
}

func (b *Boot) WithRedis() *Boot {
	b.redis = true
	return b
}

func (b *Boot) WithDb() *Boot {
	b.db = true
	return b
}

func (b *Boot) WithElastic() *Boot {
	b.elastic = true
	return b
}

func (b *Boot) WithKafka() *Boot {
	b.kafka = true
	return b
}

func (b *Boot) WithMinio() *Boot {
	b.minio = true
	return b
}

func (b *Boot) WithMongodb() *Boot {
	b.mongodb = true
	return b
}

func (b *Boot) WithRabbitmq() *Boot {
	b.rabbitmq = true
	return b
}

func (b *Boot) WithRisingwave() *Boot {
	b.risingwave = true
	return b
}

func (b *Boot) WithTimer() *Boot {
	b.timer = true
	return b
}

func (b *Boot) Options() *sharkapp.Options {
	if b.project == "" || b.name == "" || b.id == "" || b.env == "" || b.redis_host == "" || b.redis_port == "" || b.redis_password == "" {
		panic("project, name, id, env, redis_host, redis_port and redis_password are required")
	}
	options := sharkapp.NewOption()
	r, err := sharkredis.New(context.Background(), &sharkredis.Config{
		Host:     b.redis_host,
		Port:     cast.ToInt(b.redis_port),
		Password: b.redis_password,
	})
	if err != nil {
		panic(err)
	}
	if b.redis {
		value := r.Get(context.Background(), b.project+":system:config:redis").Val()
		config := &sharkredis.Config{}
		err = sonic.Unmarshal([]byte(value), config)
		if err != nil {
			panic(err)
		}
		options.WithRedis(config)
	}
	if b.db {
		value := r.Get(context.Background(), b.project+":system:config:db").Val()
		config := &sharkdb.Config{}
		err = sonic.Unmarshal([]byte(value), config)
		if err != nil {
			panic(err)
		}
		options.WithDb(config)
	}
	if b.elastic {
		value := r.Get(context.Background(), b.project+":system:config:elastic").Val()
		config := &sharkelastic.Config{}
		err = sonic.Unmarshal([]byte(value), config)
		if err != nil {
			panic(err)
		}
		options.WithElastic(config)
	}
	if b.kafka {
		value := r.Get(context.Background(), b.project+":system:config:kafka").Val()
		config := &sharkkafka.Config{}
		err = sonic.Unmarshal([]byte(value), config)
		if err != nil {
			panic(err)
		}
		options.WithKafka(config)
	}
	if b.minio {
		value := r.Get(context.Background(), b.project+":system:config:minio").Val()
		config := &sharkminio.Config{}
		err = sonic.Unmarshal([]byte(value), config)
		if err != nil {
			panic(err)
		}
		options.WithMinio(config)
	}
	if b.mongodb {
		value := r.Get(context.Background(), b.project+":system:config:mongodb").Val()
		config := &sharkmongodb.Config{}
		err = sonic.Unmarshal([]byte(value), config)
		if err != nil {
			panic(err)
		}
		options.WithMongodb(config)
	}
	if b.rabbitmq {
		value := r.Get(context.Background(), b.project+":system:config:rabbitmq").Val()
		config := &sharkrabbitmq.Config{}
		err = sonic.Unmarshal([]byte(value), config)
		if err != nil {
			panic(err)
		}
		options.WithRabbitmq(config)
	}
	if b.risingwave {
		value := r.Get(context.Background(), b.project+":system:config:risingwave").Val()
		config := &sharkrisingwave.Config{}
		err = sonic.Unmarshal([]byte(value), config)
		if err != nil {
			panic(err)
		}
		options.WithRisingwave(config)
	}
	if b.timer {
		options.WithTimer(true)
	}
	options = options.WithProject(b.project).WithName(b.name).WithId(b.id).WithEnv(b.env)
	value := r.Get(context.Background(), fmt.Sprintf("%v:system:config:%v-%v", b.project, b.name, b.id)).Val()
	mvalue := make(map[string]any)
	err = sonic.Unmarshal([]byte(value), &mvalue)
	if err != nil {
		panic(err)
	}
	if v, ok := mvalue["pprof"]; ok {
		options.WithPprof(cast.ToInt(v))
	}
	if v, ok := mvalue["health"]; ok {
		options.WithHealth(cast.ToInt(v))
	}
	if v, ok := mvalue["grpc"]; ok {
		options.WithGrpc(cast.ToInt(v))
	}
	if v, ok := mvalue["http"]; ok {
		options.WithHttp(cast.ToInt(v))
	}
	return options
}
