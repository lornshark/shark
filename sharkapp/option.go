package sharkapp

import (
	"shark/sharkkafka"
	"shark/sharkredis"
)

type Options struct {
	project string
	name    string
	id      string
	env     string
	kafka   *sharkkafka.Config
	redis   *sharkredis.Config
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
