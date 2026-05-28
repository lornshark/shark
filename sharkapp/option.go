package sharkapp

import (
	"github.com/lornshark/shark/sharkdb"
	"github.com/lornshark/shark/sharkelastic"
	"github.com/lornshark/shark/sharkkafka"
	"github.com/lornshark/shark/sharkminio"
	"github.com/lornshark/shark/sharkmongodb"
	"github.com/lornshark/shark/sharkrabbitmq"
	"github.com/lornshark/shark/sharkredis"
	"github.com/lornshark/shark/sharkrisingwave"
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
	grpcport   int
	checkport  int
	risingwave *sharkrisingwave.Config
}

func NewOption() *Options {
	return &Options{}
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

func (s *Options) WithHealthCheck(port int) *Options {
	s.checkport = port
	return s
}

func (s *Options) WithGrpc(port int) *Options {
	s.grpcport = port
	return s
}
