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
// 所有字段均为小写（私有），外部只能通过 WithXxx 方法链式设置，
// 或通过 NewOption / NewOptionWithRedis 从配置文件 / Redis 中加载。
// 这种设计保证了配置来源的统一性和可控性。
type Options struct {
	// id 实例 ID，用于区分同一服务的多个实例（如多节点部署时区分节点编号）
	id string
	// db MySQL/GORM 数据库配置
	db *sharkdb.Config
	// env 运行环境：dev（开发）、test（测试）、prod（生产）
	env string
	// name 服务名称，如 "game-server"、"user-service"
	name string
	// minio MinIO 对象存储配置
	minio *sharkminio.Config
	// timer 是否启用定时器功能（依赖 Redis）
	timer bool
	// pprof 性能分析端口，>0 时启用 pprof HTTP 服务
	pprof int
	// kafka Kafka 消息队列配置
	kafka *sharkkafka.Config
	// redis Redis（兼容 cluster/client 模式）配置
	redis *sharkredis.Config
	// redis_cluster Redis 集群模式配置，优先级高于 redis_client
	redis_cluster *sharkredis.Config
	// redis_client Redis 单机/主从模式配置
	redis_client *sharkredis.Config
	// elastic Elasticsearch 配置
	elastic *sharkelastic.Config
	// project 项目名称，用于日志 topic、定时器 key 等命名空间隔离
	project string
	// mongodb MongoDB 配置
	mongodb *sharkmongodb.Config
	// rabbitmq RabbitMQ 配置
	rabbitmq *sharkrabbitmq.Config
	// grpc gRPC 服务端口，>0 时启用，<0 时初始化但不启动监听
	grpc int
	// health 健康检查 HTTP 服务端口，>0 时启用 /health 端点
	health int
	// risingwave RisingWave 流数据库配置
	risingwave *sharkrisingwave.Config
	// etcd etcd 配置（分布式配置中心/服务发现）
	etcd *sharketcd.Config
	// http HTTP 服务端口（基于 Gin），>0 时启用
	http int
}

// read_slices 从 viper 中读取字符串切片配置。
//
// 支持两种格式：
//  1. YAML 数组格式：key: ["a", "b", "c"]
//  2. YAML 字符串逗号分隔格式：key: "a, b, c"
//
// 如果 GetStringSlice 没有获取到值，则回退到 GetString 并用逗号分隔解析。
// 所有元素会去除首尾空白，空字符串会被过滤。
//
// 参数:
//   - v: viper 配置实例
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
//
// 参数:
//   - project: 项目名称，用于日志 topic 等命名空间隔离
//   - name: 服务名称
//
// 返回值:
//   - *Options: 包含所有配置的选项对象，后续可通过 WithXxx 方法覆盖
//
// 注意事项:
//   - 如果 config.yaml 文件不存在，不会报错（视为无配置文件），所有配置使用默认值
//   - 如果 config.yaml 存在但格式错误，会 panic
//   - 环境变量优先级高于配置文件
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
	// redis_cluster 集群模式配置
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
	// redis_client 单机/主从模式配置
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
	// redis 兼容模式配置（自动探测 cluster/client）
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
	// db MySQL 数据库配置
	{
		hosts := options.read_slices(v, "db.host")
		if len(hosts) > 0 {
			options.db = &sharkdb.Config{
				Host:     hosts[0], // 数据库只取第一个 host（不支持多地址）
				User:     strings.TrimSpace(v.GetString("db.user")),
				Password: strings.TrimSpace(v.GetString("db.password")),
				Database: strings.TrimSpace(v.GetString("db.database")),
			}
		}
	}
	// elastic Elasticsearch 配置
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
	// minio MinIO 对象存储配置
	{
		hosts := options.read_slices(v, "minio.host")
		if len(hosts) > 0 {
			options.minio = &sharkminio.Config{
				Host:     hosts[0], // MinIO 只取第一个 host
				User:     strings.TrimSpace(v.GetString("minio.user")),
				Password: strings.TrimSpace(v.GetString("minio.password")),
			}
		}
	}
	// kafka Kafka 消息队列配置
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
	// mongodb MongoDB 配置
	{
		hosts := options.read_slices(v, "mongodb.host")
		if len(hosts) > 0 {
			options.mongodb = &sharkmongodb.Config{
				Host:     hosts[0], // MongoDB 只取第一个 host
				User:     strings.TrimSpace(v.GetString("mongodb.user")),
				Password: strings.TrimSpace(v.GetString("mongodb.password")),
			}
		}
	}
	// rabbitmq RabbitMQ 配置
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
	// risingwave RisingWave 流数据库配置
	{
		hosts := options.read_slices(v, "risingwave.host")
		if len(hosts) > 0 {
			options.risingwave = &sharkrisingwave.Config{
				Host:     hosts[0], // RisingWave 只取第一个 host
				User:     strings.TrimSpace(v.GetString("risingwave.user")),
				Password: strings.TrimSpace(v.GetString("risingwave.password")),
				Database: strings.TrimSpace(v.GetString("risingwave.database")),
			}
		}
	}
	// etcd 配置
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
