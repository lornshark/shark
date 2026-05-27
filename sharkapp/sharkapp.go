package sharkapp

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"shark/sharkdb"
	"shark/sharkelastic"
	"shark/sharkkafka"
	"shark/sharklog"
	"shark/sharkrabbitmq"
	"shark/sharkredis"
	"shark/sharkrisingwave"
	"shark/sharktimer"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/olivere/elastic/v7"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

type App struct {
	Project    string
	Name       string
	Id         string
	Env        string
	Running    *atomic.Bool
	Wg         *sync.WaitGroup
	Sg         *singleflight.Group
	sharklog   *sharklog.SharkLog
	Kafka      *sharkkafka.SharkKafka
	Redis      *redis.ClusterClient
	Logger     *zap.Logger
	Db         *gorm.DB
	RisingWave *gorm.DB
	Rabbitmq   *sharkrabbitmq.Client
	Elastic    *elastic.Client
	Timer      *sharktimer.Timer
	mux        *http.ServeMux
}

func New(options *Options) (*App, error) {
	app := &App{
		Project: options.project,
		Name:    options.name,
		Id:      options.id,
		Env:     options.env,
		Running: &atomic.Bool{},
		Wg:      &sync.WaitGroup{},
		Sg:      &singleflight.Group{},
	}
	app.Running.Store(true)
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
	if options.db != nil {
		db, err := sharkdb.NewDb(app.Logger, options.db)
		if err != nil {
			app.Logger.Error("连接db失败", zap.String("host", options.db.Host), zap.Int("port", options.db.Port), zap.String("database", options.db.Database), zap.Error(err))
			return nil, err
		}
		app.Logger.Info("连接db成功", zap.String("host", options.db.Host), zap.Int("port", options.db.Port), zap.String("database", options.db.Database))
		app.Db = db
	}
	if options.elastic != nil {
		elastic, err := sharkelastic.New(options.elastic)
		if err != nil {
			app.Logger.Error("连接elastic失败", zap.String("host", options.elastic.Host), zap.Error(err))
			return nil, err
		}
		app.Logger.Info("连接elastic成功", zap.String("host", options.elastic.Host))
		app.Elastic = elastic
	}
	if options.rabbitmq != nil {
		options.rabbitmq.Logger = app.Logger
		options.rabbitmq.Wg = app.Wg
		options.rabbitmq.Running = app.Running
		mq, err := sharkrabbitmq.New(options.rabbitmq)
		if err != nil {
			app.Logger.Error("连接rabbitmq失败", zap.Strings("host", options.rabbitmq.Host), zap.Error(err))
			return nil, err
		}
		app.Logger.Info("连接rabbitmq成功", zap.Strings("host", options.rabbitmq.Host))
		app.Rabbitmq = mq
	}
	if options.risingwave != nil {
		rw, err := sharkrisingwave.New(app.Logger, options.risingwave)
		if err != nil {
			app.Logger.Error("连接risingwave失败", zap.String("host", options.risingwave.Host), zap.Int("port", options.risingwave.Port), zap.String("database", options.risingwave.Database), zap.Error(err))
			return nil, err
		}
		app.Logger.Info("连接risingwave成功", zap.String("host", options.risingwave.Host), zap.Int("port", options.risingwave.Port), zap.String("database", options.risingwave.Database))
		app.RisingWave = rw
	}
	if options.timer && app.Redis != nil {
		app.Timer = sharktimer.NewTimer(app.Project, app.Name, app.Id, app.Redis)
	}
	if options.pprof > 0 {
		app.mux = http.NewServeMux()
		app.mux.HandleFunc("/debug/pprof/", pprof.Index)
		app.mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		app.mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		app.mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		app.mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		go app.pprof(options.pprof)
	}
	if options.checkport > 0 {
		go app.health_service(options.checkport)
	}
	return app, nil
}

func (a *App) pprof(port int) {
	a.Logger.Info("开启pprof服务", zap.Int("port", port))
	err := http.ListenAndServe(fmt.Sprintf(":%v", port), a.mux)
	if err != nil {
		a.Logger.Error("pprof服务启动失败", zap.Int("port", port), zap.Error(err))
	}
}

func (a *App) health_service(port int) {
	http.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(200)
		writer.Write([]byte(`{"status": "ok"}`))
	})
	a.Logger.Info("开启健康检查服务", zap.Int("port", port))
	err := http.ListenAndServe(fmt.Sprintf(":%v", port), nil)
	if err != nil {
		a.Logger.Error("健康检查服务启动失败", zap.Int("port", port), zap.Error(err))
	}
}

func (a *App) Hunt() {
	time.Sleep(time.Millisecond * 100)
	a.banner()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM)
	signal.Notify(sig, syscall.SIGINT)
	<-sig
	a.Running.Store(false)
	if a.Rabbitmq != nil {
		a.Rabbitmq.Exit()
	}
	a.sharklog.Close()
	time.Sleep(time.Millisecond * 500)
	a.Wg.Wait()
	a.Logger.Debug("****************server exit****************")
}

func (a *App) banner() {
	fmt.Println("███████╗██╗  ██╗ █████╗ ██████╗ ██╗  ██╗")
	fmt.Println("██╔════╝██║  ██║██╔══██╗██╔══██╗██║ ██╔╝")
	fmt.Println("███████╗███████║███████║██████╔╝█████╔╝")
	fmt.Println("╚════██║██╔══██║██╔══██║██╔══██╗██╔═██╗")
	fmt.Println("███████║██║  ██║██║  ██║██║  ██╗██║  ██╗")
	fmt.Println("╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝")
	// fmt.Println("")
	// fmt.Println("              _oo0oo_")
	// fmt.Println("             o8888888o")
	// fmt.Println("             88\" . \"88")
	// fmt.Println("             (| -_- |)")
	// fmt.Println("             0\\  =  /0")
	// fmt.Println("           ___/---\\___")
	// fmt.Println("         .' \\\\|     |// '.")
	// fmt.Println("        / \\\\\\|||  :  |||// \\")
	// fmt.Println("       / _||||| -:- |||||- \\")
	// fmt.Println("      |   | \\\\\\  -  /// |   |")
	// fmt.Println("      | \\_|  ''\\---/''  |_/ |")
	// fmt.Println("      \\  .-\\__  '-'  ___/-. /")
	// fmt.Println("    ___'. .'  /--.--\\  `. .'___")
	// fmt.Println(" .\"\" '<   .___\\_<|>_/___.' > \"\".")
	// fmt.Println("| | :  '- \\.;\\ _ /;./ - ' : | |")
	// fmt.Println("\\  \\ _.   \\_ __\\ /__ _/   .-  /")
	// fmt.Println("====='-.____.___ \\_____/___.-=====")
	// fmt.Println("              =---=")
	// fmt.Println("")
	// fmt.Println("           佛祖保佑 永无 BUG")
}
