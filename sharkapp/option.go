package sharkapp

import (
	"strings"

	"github.com/lornshark/shark/sharkdb"
	"github.com/lornshark/shark/sharkelastic"
	"github.com/lornshark/shark/sharketcd"
	"github.com/lornshark/shark/sharkkafka"
	"github.com/lornshark/shark/sharkminio"
	"github.com/lornshark/shark/sharkmongodb"
	"github.com/lornshark/shark/sharkrabbitmq"
	"github.com/lornshark/shark/sharkredis"
	"github.com/lornshark/shark/sharkrisingwave"
	"github.com/spf13/viper"
)

// Options 是应用的配置集合，包含了所有中间件的连接配置。
//
// 所有字段均为小写（私有），外部只能通过 NewOption() / NewOptionWithRedis() 加载，
// 或通过 WithXxx 方法链式覆盖单个配置项。
// 这种设计保证了配置来源的统一性和可控性。
//
// 配置加载优先级（由高到低）：
//  1. 环境变量（如 REDIS_HOST 覆盖 redis.host）
//  2. config.yaml 配置文件
//  3. 代码默认值
//
// 使用示例：
//
//	// 从本地配置文件加载
//	opts := sharkapp.NewOption("myproject", "game-server")
//
//	// 链式覆盖部分配置
//	opts.WithDB(&sharkdb.Config{Host: "custom-db:3306", ...}).
//	    WithRedis(&sharkredis.Config{Host: []string{"cache:6379"}, ...})
//
//	// 传递给应用启动
//	app, err := sharkapp.New(opts)
type Options struct {
	// id 实例 ID，用于区分同一服务的多个实例（如多节点部署时区分节点编号）
	// 对应 config.yaml 中的 id 字段，默认值 "1"
	id string

	// db MySQL/GORM 数据库配置
	// 配置 key: db.host / db.user / db.password / db.database
	db *sharkdb.Config

	// env 运行环境标识
	// 可选值: "dev"（开发）、"test"（测试）、"prod"（生产）
	// 对应 config.yaml 中的 env 字段，默认值 "dev"
	env string

	// name 服务名称，如 "game-server"、"user-service"
	// 通过 NewOption 的第二个参数传入
	name string

	// minio MinIO 对象存储配置
	// 用于文件存储（图片、文档、日志等）
	minio *sharkminio.Config

	// timer 是否启用定时器功能（依赖 Redis）
	// true = 启用 sharktimer 后台定时任务轮询
	// false = 禁用
	timer bool

	// pprof 性能分析端口，>0 时启用 pprof HTTP 服务
	// 通常仅在开发/调试环境开启
	pprof int

	// kafka Kafka 消息队列配置
	// 用于异步消息通信、事件溯源、日志推送
	kafka *sharkkafka.Config

	// redis Redis 兼容模式配置（自动探测集群/单机模式）
	// 优先级低于 redis_cluster 和 redis_client
	redis *sharkredis.Config

	// redis_cluster Redis 集群模式配置
	// 优先级高于 redis_client
	redis_cluster *sharkredis.Config

	// redis_client Redis 单机/主从模式配置
	redis_client *sharkredis.Config

	// elastic Elasticsearch 搜索引擎配置
	// 用于全文检索、日志分析、数据聚合查询
	elastic *sharkelastic.Config

	// project 项目名称，用于日志 topic、定时器 key 等命名空间隔离
	// 通过 NewOption 的第一个参数传入
	project string

	// mongodb MongoDB 文档数据库配置
	// 用于非结构化数据存储
	mongodb *sharkmongodb.Config

	// rabbitmq RabbitMQ 消息队列配置
	// 用于消息消费（如订单处理、通知推送等）
	rabbitmq *sharkrabbitmq.Config

	// grpc gRPC 服务端口
	// >0: 启用 gRPC 服务端监听
	// =0: 不启用 gRPC 服务端（仅作客户端）
	grpc int

	// health 健康检查 HTTP 服务端口
	// >0 时启用 /health 端点（供 K8s 等基础设施探测）
	health int

	// risingwave RisingWave 流数据库配置
	// 兼容 PostgreSQL 协议，用于物化视图和实时数据处理
	risingwave *sharkrisingwave.Config

	// etcd etcd 分布式键值存储配置
	// 用于分布式配置管理、服务注册与发现、分布式协调
	etcd *sharketcd.Config

	// http HTTP 服务端口（基于 Gin）
	// >0 时启用 REST API 服务
	http int
}

// read_slices 从 viper 中读取字符串切片配置。
//
// 支持两种 YAML 格式：
//  1. 数组格式：key: ["a", "b", "c"]
//  2. 逗号分隔字符串格式：key: "a, b, c"
//
// 处理逻辑：
//  1. 优先尝试 GetStringSlice（YAML 数组格式）
//  2. 对数组中的每个元素再做逗号分隔（兼容 "a,b" 这种混合写法）
//  3. 若数组方式无数据，回退到 GetString + 逗号分隔解析
//  4. 所有元素去除首尾空白，空字符串被过滤
//
// 参数:
//   - v:   viper 配置实例
//   - key: 配置键名（支持点号分隔的嵌套路径，如 "redis.host"）
//
// 返回值:
//   - 去重去空后的字符串切片
func (o *Options) read_slices(v *viper.Viper, key string) []string {
	// 首先尝试通过 GetStringSlice 获取（支持 YAML 数组格式）
	ss := v.GetStringSlice(key)
	var result []string
	for _, s := range ss {
		// 对数组中的每个元素再做逗号分隔（兼容 "a,b" 这种写法）
		for _, sub := range strings.Split(s, ",") {
			if t := strings.TrimSpace(sub); t != "" {
				result = append(result, t)
			}
		}
	}
	// 如果数组方式没有获取到值，回退到字符串 + 逗号分隔方式
	if len(result) == 0 {
		if s := strings.TrimSpace(v.GetString(key)); s != "" {
			for _, sub := range strings.Split(s, ",") {
				if t := strings.TrimSpace(sub); t != "" {
					result = append(result, t)
				}
			}
		}
	}
	return result
}

// NewOption 从本地 YAML 配置文件（config.yaml）中加载应用配置。
//
// 配置加载流程:
//  1. 创建 viper 实例，读取当前目录或 ./config 目录下的 config.yaml
//  2. 同时支持环境变量覆盖（环境变量中 . 替换为 _，如 redis.host → REDIS_HOST）
//  3. 按照统一的 key 格式解析各中间件的连接信息
//  4. 仅当 host 列表非空时才创建对应的 Config 实例，避免无效连接
//
// 参数:
//   - project: 项目名称，用于日志 topic、定时器 key 等命名空间隔离
//   - name:    服务名称，用于服务标识
//
// 返回值:
//   - *Options: 包含完整配置的选项对象，后续可通过 WithXxx 方法覆盖单项配置
//
// 注意事项:
//   - 如果 config.yaml 文件不存在，不会报错（视为无配置文件），所有中间件配置为空
//   - 如果 config.yaml 存在但 YAML 格式错误，会 panic
//   - 环境变量优先级高于配置文件
//
// 使用示例：
//
//	// 基础用法：从 config.yaml 加载
//	opts := sharkapp.NewOption("myproject", "game-server")
//
//	// 如果 config.yaml 不存在，所有中间件配置为空，需手动设置
//	opts := sharkapp.NewOption("myproject", "game-server").
//	    WithDB(&sharkdb.Config{
//	        Host:     "127.0.0.1:3306",
//	        User:     "root",
//	        Password: "secret",
//	        Database: "mydb",
//	    }).WithRedis(&sharkredis.Config{
//	        Host:     []string{"127.0.0.1:6379"},
//	        Password: "",
//	    })
//
//	// 启动应用
//	app, err := sharkapp.New(opts)
//	if err != nil {
//	    panic(err)
//	}
func NewOption(project string, name string) *Options {
	v := viper.New()
	// 配置文件名和类型
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	// 搜索路径：当前目录和 ./config 子目录
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	// 默认值
	v.SetDefault("env", "dev")
	v.SetDefault("id", "1")
	// 环境变量映射：将配置 key 中的 . 替换为 _ 后查找环境变量
	// 例如 redis.host -> REDIS_HOST
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		// 配置文件不存在是允许的，但其他错误（如格式错误）需要 panic
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			panic(err)
		}
	}
	// 读取基础配置
	env := strings.TrimSpace(v.GetString("env"))
	id := strings.TrimSpace(v.GetString("id"))
	options := &Options{
		project: project,
		name:    name,
		env:     env,
		id:      id,
	}
	// 读取功能开关和端口配置
	options.timer = v.GetBool("timer")
	options.pprof = v.GetInt("pprof")
	options.grpc = v.GetInt("grpc")
	options.health = v.GetInt("health")
	options.http = v.GetInt("http")

	// 以下使用独立作用域块（{}）来隔离各中间件的配置解析
	// 每个块内：读取 host 列表 → 非空时创建对应的 Config 实例

	// ========== Redis 集群模式配置 ==========
	// 配置 key 前缀: redis_cluster
	{
		hosts := options.read_slices(v, "redis_cluster.host")
		if len(hosts) > 0 {
			options.redis_cluster = &sharkredis.Config{
				Host:        hosts,
				Password:    strings.TrimSpace(v.GetString("redis_cluster.password")),
				ReplaceFrom: strings.TrimSpace(v.GetString("redis_cluster.replace_from")),
				ReplaceTo:   strings.TrimSpace(v.GetString("redis_cluster.replace_to")),
			}
		}
	}
	// ========== Redis 单机/主从模式配置 ==========
	// 配置 key 前缀: redis_client
	{
		hosts := options.read_slices(v, "redis_client.host")
		if len(hosts) > 0 {
			options.redis_client = &sharkredis.Config{
				Host:        hosts,
				Password:    strings.TrimSpace(v.GetString("redis_client.password")),
				ReplaceFrom: strings.TrimSpace(v.GetString("redis_client.replace_from")),
				ReplaceTo:   strings.TrimSpace(v.GetString("redis_client.replace_to")),
			}
		}
	}
	// ========== Redis 兼容模式配置（自动探测集群/单机）==========
	// 配置 key 前缀: redis
	{
		hosts := options.read_slices(v, "redis.host")
		if len(hosts) > 0 {
			options.redis = &sharkredis.Config{
				Host:        hosts,
				Password:    strings.TrimSpace(v.GetString("redis.password")),
				ReplaceFrom: strings.TrimSpace(v.GetString("redis.replace_from")),
				ReplaceTo:   strings.TrimSpace(v.GetString("redis.replace_to")),
			}
		}
	}
	// ========== MySQL 数据库配置 ==========
	// 配置 key 前缀: db
	// 注意：数据库只取第一个 host（不支持多地址负载均衡）
	{
		hosts := options.read_slices(v, "db.host")
		if len(hosts) > 0 {
			options.db = &sharkdb.Config{
				Host:     hosts[0],
				User:     strings.TrimSpace(v.GetString("db.user")),
				Password: strings.TrimSpace(v.GetString("db.password")),
				Database: strings.TrimSpace(v.GetString("db.database")),
			}
		}
	}
	// ========== Elasticsearch 配置 ==========
	// 配置 key 前缀: elastic
	// 支持多地址（集群模式）
	{
		hosts := options.read_slices(v, "elastic.host")
		if len(hosts) > 0 {
			options.elastic = &sharkelastic.Config{
				Host:     hosts,
				User:     strings.TrimSpace(v.GetString("elastic.user")),
				Password: strings.TrimSpace(v.GetString("elastic.password")),
			}
		}
	}
	// ========== MinIO 对象存储配置 ==========
	// 配置 key 前缀: minio
	// 注意：只取第一个 host
	{
		hosts := options.read_slices(v, "minio.host")
		if len(hosts) > 0 {
			options.minio = &sharkminio.Config{
				Host:     hosts[0],
				User:     strings.TrimSpace(v.GetString("minio.user")),
				Password: strings.TrimSpace(v.GetString("minio.password")),
			}
		}
	}
	// ========== Kafka 消息队列配置 ==========
	// 配置 key 前缀: kafka
	// 支持多 Broker 地址
	{
		hosts := options.read_slices(v, "kafka.host")
		if len(hosts) > 0 {
			options.kafka = &sharkkafka.Config{
				Host:     hosts,
				User:     strings.TrimSpace(v.GetString("kafka.user")),
				Password: strings.TrimSpace(v.GetString("kafka.password")),
			}
		}
	}
	// ========== MongoDB 文档数据库配置 ==========
	// 配置 key 前缀: mongodb
	// 注意：只取第一个 host
	{
		hosts := options.read_slices(v, "mongodb.host")
		if len(hosts) > 0 {
			options.mongodb = &sharkmongodb.Config{
				Host:     hosts[0],
				User:     strings.TrimSpace(v.GetString("mongodb.user")),
				Password: strings.TrimSpace(v.GetString("mongodb.password")),
			}
		}
	}
	// ========== RabbitMQ 消息队列配置 ==========
	// 配置 key 前缀: rabbitmq
	// 支持多 Broker 地址（逗号分隔）
	{
		hosts := options.read_slices(v, "rabbitmq.host")
		if len(hosts) > 0 {
			options.rabbitmq = &sharkrabbitmq.Config{
				Host:     hosts,
				User:     strings.TrimSpace(v.GetString("rabbitmq.user")),
				Password: strings.TrimSpace(v.GetString("rabbitmq.password")),
			}
		}
	}
	// ========== RisingWave 流数据库配置 ==========
	// 配置 key 前缀: risingwave
	// 注意：只取第一个 host
	{
		hosts := options.read_slices(v, "risingwave.host")
		if len(hosts) > 0 {
			options.risingwave = &sharkrisingwave.Config{
				Host:     hosts[0],
				User:     strings.TrimSpace(v.GetString("risingwave.user")),
				Password: strings.TrimSpace(v.GetString("risingwave.password")),
				Database: strings.TrimSpace(v.GetString("risingwave.database")),
			}
		}
	}
	// ========== etcd 分布式键值存储配置 ==========
	// 配置 key 前缀: etcd
	// 支持多地址（集群模式）
	{
		hosts := options.read_slices(v, "etcd.host")
		if len(hosts) > 0 {
			options.etcd = &sharketcd.Config{
				Host:     hosts,
				User:     strings.TrimSpace(v.GetString("etcd.user")),
				Password: strings.TrimSpace(v.GetString("etcd.password")),
			}
		}
	}
	return options
}
