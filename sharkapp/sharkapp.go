package sharkapp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/howeyc/crc16"
	"github.com/lornshark/shark/sharkdb"
	"github.com/lornshark/shark/sharkelastic"
	"github.com/lornshark/shark/sharkgrpc"
	"github.com/lornshark/shark/sharkhttp"
	"github.com/lornshark/shark/sharkkafka"
	"github.com/lornshark/shark/sharkminio"
	"github.com/lornshark/shark/sharkmongodb"
	"github.com/lornshark/shark/sharkrabbitmq"
	"github.com/lornshark/shark/sharkredis"
	"github.com/lornshark/shark/sharkrisingwave"

	"github.com/lornshark/shark/sharklog"
	"github.com/lornshark/shark/sharktimer"

	"github.com/minio/minio-go/v7"
	"github.com/olivere/elastic/v7"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

// 使用示例 使用 redis 作为配置工具:
//  project := "kgame"
// 	name := "game-test"
// 	id := "1"
// 	options := sharkapp.NewOptionWithRedis("dev", project, name, id, "192.168.191.100", "6379", "CEki57pxTJyYaLD")
// 	options.WithDb(nil).WithRedis(nil).WithKafka(nil).WithElastic(nil).
// 		WithMongodb(nil).WithRabbitmq(nil).WithRisingwave(nil)
// 	app, err := sharkapp.New(options)
// 	if err != nil {
// 		panic(err)
// 	}
// 	app.Hunt()

// 也可以直接创建 App 实例，不使用配置工具，
//
//	options := sharkapp.NewOption("dev", project, name, id)
//  options.WithDb(&sharkdb.Config{Host: "", Port: 3306, User: "", Password: "", Database: ""}).WithRedis(&sharkredis.Config{Host: "", Port: 6379, Password: ""}).
//		WithKafka(&sharkkafka.Config{Host: "", Port: 9092}).
//		WithElastic(&sharkelastic.Config{Host: "", Port: 9200}).
//		WithMongodb(&sharkmongodb.Config{Host: "", Port: 27017}).
//		WithRabbitmq(&sharkrabbitmq.Config{Host: []string{""}, Port: 5672, User: "", Password: ""}).
//		WithRisingwave(&sharkrisingwave.Config{Host: "", Port: 3306, User: "", Password: "", Database: ""})
// 	app, err := sharkapp.New(options)
// 	if err != nil {
// 		panic(err)
// 	}
// 	app.Hunt()

// 组件单独使用
// redis, err := sharkredis.New(ctx, &sharkredis.Config{Host: "", Port: 6379, Password: ""})
// mongo,err := sharkmongodb.New(ctx, &sharkmongodb.Config{Host: "", Port: 27017})
// redis,err := sharkredis.New(ctx, &sharkredis.Config{Host: "", Port: 6379, Password: ""})

type App struct {
	// 生命周期
	Wg      *sync.WaitGroup
	Sg      *singleflight.Group
	Context context.Context
	// 基础信息
	Id      string
	Env     string
	Name    string
	Project string
	// 基础组件
	Db         *gorm.DB
	Grpc       *sharkgrpc.RpcServer
	Minio      *minio.Client
	Timer      *sharktimer.Timer
	Kafka      *sharkkafka.SharkKafka
	Redis      *redis.ClusterClient
	Logger     *zap.Logger
	Elastic    *elastic.Client
	Mongodb    *mongo.Client
	Rabbitmq   *sharkrabbitmq.Client
	RisingWave *gorm.DB
	GinEngine  *gin.Engine
	// 内部组件
	cancelFunc context.CancelFunc
	sharklog   *sharklog.SharkLog
	muxServe   *http.ServeMux
}

func New(options *Options) (*App, error) {
	ctx, cancel := context.WithCancel(context.Background())
	app := &App{
		Context:    ctx,
		cancelFunc: cancel,
		Project:    options.project,
		Name:       options.name,
		Id:         options.id,
		Env:        options.env,
		Wg:         &sync.WaitGroup{},
		Sg:         &singleflight.Group{},
	}
	app.sharklog = sharklog.New(app.Context, app.Name, app.Id)
	app.Logger = app.sharklog.Zap
	if options.kafka != nil {
		kafka, err := sharkkafka.New(app.Context, options.kafka, app.Logger)
		if err != nil {
			return nil, err
		}
		app.Kafka = kafka
		kafkaLogWriter, _ := app.Kafka.Writer(fmt.Sprintf("%v_game_log", app.Project))
		app.sharklog.SetKafkaWriter(kafkaLogWriter)
	}
	if options.kafka != nil {
		app.Logger.Info("连接kafka成功", zap.String("host", options.kafka.Host), zap.Int("port", options.kafka.Port))
	}
	if options.redis != nil {
		redis, err := sharkredis.New(app.Context, options.redis)
		if err != nil {
			app.Logger.Error("连接redis失败", zap.String("host", options.redis.Host), zap.Int("port", options.redis.Port), zap.Error(err))
			return nil, err
		}
		app.Logger.Info("连接redis成功", zap.String("host", options.redis.Host), zap.Int("port", options.redis.Port))
		app.Redis = redis
	}
	if options.db != nil {
		db, err := sharkdb.NewDb(app.Context, app.Logger, options.db)
		if err != nil {
			app.Logger.Error("连接db失败", zap.String("host", options.db.Host), zap.Int("port", options.db.Port), zap.String("database", options.db.Database), zap.Error(err))
			return nil, err
		}
		app.Logger.Info("连接db成功", zap.String("host", options.db.Host), zap.Int("port", options.db.Port), zap.String("database", options.db.Database))
		app.Db = db
	}
	if options.elastic != nil {
		elastic, err := sharkelastic.New(app.Context, options.elastic)
		if err != nil {
			app.Logger.Error("连接elastic失败", zap.String("host", options.elastic.Host), zap.Error(err))
			return nil, err
		}
		app.Logger.Info("连接elastic成功", zap.String("host", options.elastic.Host))
		app.Elastic = elastic
	}
	if options.rabbitmq != nil {
		mq, err := sharkrabbitmq.New(app.Context, app.Logger, app.Wg, options.rabbitmq, app.Name, app.Id)
		if err != nil {
			app.Logger.Error("连接rabbitmq失败", zap.Strings("host", options.rabbitmq.Host), zap.Error(err))
			return nil, err
		}
		index := crc16.Checksum([]byte(app.Name), crc16.IBMTable)
		index = index % uint16(len(options.rabbitmq.Host))
		app.Logger.Info("连接rabbitmq成功", zap.String("host", options.rabbitmq.Host[index]))
		app.Rabbitmq = mq
	}
	if options.risingwave != nil {
		rw, err := sharkrisingwave.New(app.Context, app.Logger, options.risingwave)
		if err != nil {
			app.Logger.Error("连接risingwave失败", zap.String("host", options.risingwave.Host), zap.Int("port", options.risingwave.Port), zap.String("database", options.risingwave.Database), zap.Error(err))
			return nil, err
		}
		app.Logger.Info("连接risingwave成功", zap.String("host", options.risingwave.Host), zap.Int("port", options.risingwave.Port), zap.String("database", options.risingwave.Database))
		app.RisingWave = rw
	}
	if options.mongodb != nil {
		mongodb, err := sharkmongodb.New(app.Context, options.mongodb)
		if err != nil {
			app.Logger.Error("连接mongodb失败", zap.String("host", options.mongodb.Host), zap.Int("port", options.mongodb.Port), zap.Error(err))
			return nil, err
		}
		app.Logger.Info("连接mongodb成功", zap.String("host", options.mongodb.Host), zap.Int("port", options.mongodb.Port))
		app.Mongodb = mongodb
	}
	if options.minio != nil {
		client, err := sharkminio.New(app.Context, options.minio)
		if err != nil {
			app.Logger.Error("连接minio失败", zap.String("host", options.minio.Host), zap.Int("port", options.minio.Port), zap.Error(err))
			return nil, err
		}
		app.Logger.Info("连接minio成功", zap.String("host", options.minio.Host), zap.Int("port", options.minio.Port))
		app.Minio = client
	}
	if options.timer {
		if app.Redis != nil {
			app.Timer = sharktimer.NewTimer(app.Context, app.Project, app.Name, app.Id, app.Redis)
			app.Logger.Info("初始化timer成功")
		} else {
			app.Logger.Error("初始化timer失败, 依赖redis, 请确保已正确配置redis连接")
		}
	}
	if options.grpc > 0 {
		if app.Redis != nil {
			server := sharkgrpc.New(app.Context, app.Project, app.Redis, app.Logger, options.grpc)
			app.Logger.Info("开启rpc服务", zap.Int("port", options.grpc))
			app.Grpc = server
		} else {
			app.Logger.Error("开启rpc服务失败, 依赖redis, 请确保已正确配置redis连接")
		}
	}
	if options.http > 0 {
		app.GinEngine = sharkhttp.New(app.Context, app.Env, app.Logger, options.http)
		app.Logger.Info("开启http服务", zap.Int("port", options.http))
		if options.env == "dev" {
			app.Logger.Debug("swagger url: http://127.0.0.1" + ":" + fmt.Sprint(options.http) + "/swagger/index.html")
		}
	}
	if options.pprof > 0 {
		go app.pprof(options.pprof)
	}
	if options.health > 0 {
		go app.health_service(options.health)
	}
	return app, nil
}

func (a *App) pprof(port int) {
	a.Logger.Info("开启pprof服务", zap.Int("port", port))
	a.muxServe = http.NewServeMux()
	a.muxServe.HandleFunc("/debug/pprof/", pprof.Index)
	err := http.ListenAndServe(fmt.Sprintf(":%v", port), a.muxServe)
	if err != nil {
		a.Logger.Error("pprof服务启动失败", zap.Int("port", port), zap.Error(err))
	}
}

func (a *App) health_service(port int) {
	http.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(200)
		writer.Header().Set("Content-Type", "application/json")
		writer.Write([]byte(`{"status": "ok"}`))
	})
	a.Logger.Info("开启健康检查服务", zap.Int("port", port))
	err := http.ListenAndServe(fmt.Sprintf(":%v", port), nil)
	if err != nil {
		a.Logger.Error("健康检查服务启动失败", zap.Int("port", port), zap.Error(err))
	}
}

type AppComponent interface {
	Init()
	Start()
}

func (a *App) Hunt(components ...AppComponent) {
	time.Sleep(time.Millisecond * 100)
	for _, c := range components {
		c.Init()
	}
	for _, c := range components {
		c.Start()
	}
	a.banner()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM)
	signal.Notify(sig, syscall.SIGINT)
	<-sig
	a.cancelFunc()
	time.Sleep(time.Millisecond * 500)
	a.Wg.Wait()
	a.Logger.Debug("****************server exit****************")
}

func (a *App) banner() {
	a.Logger.Info("****************server start****************")
	//	banner := `███████╗██╗  ██╗ █████╗ ██████╗ ██╗  ██╗    ██╗  ██╗██╗   ██╗███╗   ██╗████████╗██╗███╗   ██╗ ██████╗
	// ██╔════╝██║  ██║██╔══██╗██╔══██╗██║ ██╔╝    ██║  ██║██║   ██║████╗  ██║╚══██╔══╝██║████╗  ██║██╔════╝
	// ███████╗███████║███████║██████╔╝█████╔╝     ███████║██║   ██║██╔██╗ ██║   ██║   ██║██╔██╗ ██║██║  ███╗
	// ╚════██║██╔══██║██╔══██║██╔══██╗██╔═██╗     ██╔══██║██║   ██║██║╚██╗██║   ██║   ██║██║╚██╗██║██║   ██║
	// ███████║██║  ██║██║  ██║██║  ██║██║  ██╗    ██║  ██║╚██████╔╝██║ ╚████║   ██║   ██║██║ ╚████║╚██████╔╝
	// ╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝    ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝   ╚═╝   ╚═╝╚═╝  ╚═══╝ ╚═════╝`
	//	fmt.Println(banner)
}

func (a *App) Go(fn func(ctx context.Context)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				a.Logger.Error("panic in go routine", zap.Any("panic", r), zap.String("stack", string(debug.Stack())))
			}
		}()
		fn(a.Context)
	}()
}
