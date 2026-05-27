package sharkapp

import (
	"fmt"
	"shark/sharkkafka"
	"shark/sharklog"
	"shark/sharkredis"
	"sync"
	"sync/atomic"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

type App struct {
	Project  string
	Name     string
	Id       string
	Env      string
	Running  *atomic.Bool
	Wg       *sync.WaitGroup
	Sg       *singleflight.Group
	sharklog *sharklog.SharkLog
	Kafka    *sharkkafka.SharkKafka
	Redis    *redis.ClusterClient
	Logger   *zap.Logger
}

func NewSharkApp(options *Options) (*App, error) {
	app := &App{
		Project: options.project,
		Name:    options.name,
		Id:      options.id,
		Env:     options.env,
		Running: &atomic.Bool{},
		Wg:      &sync.WaitGroup{},
		Sg:      &singleflight.Group{},
	}
	if options.kafka != nil {
		kafka, err := sharkkafka.New(options.kafka)
		if err != nil {
			return nil, err
		}
		app.Kafka = kafka
	}
	var kafkaWriter *kafka.Writer
	if app.Kafka != nil {
		kafkaWriter, _ = app.Kafka.Writer(fmt.Sprintf("%v_game_log", app.Project))
	}
	app.sharklog = sharklog.New(app.Name, app.Id, kafkaWriter)
	app.Logger = app.sharklog.Zap
	if options.redis != nil {
		redis, err := sharkredis.New(options.redis)
		if err != nil {
			app.Logger.Error("连接redis失败", zap.String("host", options.redis.Host), zap.Int("port", options.redis.Port), zap.Error(err))
			return nil, err
		}
		app.Logger.Info("连接redis成功", zap.String("host", options.redis.Host), zap.Int("port", options.redis.Port))
		app.Redis = redis
	}
	return app, nil
}
