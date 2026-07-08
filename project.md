# Shark — Go 微服务框架

> ⚠️ **git 纪律：** 不要自动提交 git，除非我明确提出提交 git 的命令。

```
模块路径: github.com/lornshark/shark
Go 版本: 1.26+
许可证: MIT
作者: Lornshark
```

## 一句话

一站式 Go 微服务框架，通过 config.yaml 统一配置 → `sharkapp.New()` 自动初始化所有中间件 → `App.Hunt()` 启动业务模块 + 优雅关闭。

## 项目入口

`main.go` 创建 App 并注册业务组件：

```go
options, _ := sharkapp.NewOption("kgame", "game-test")
app, _ := sharkapp.New(options)
app.Hunt(&Test{svc: app})
```

## 包结构

| 包 | 用途 | 关键导出 |
|---|---|---|
| `sharkapp` | **应用启动核心** | `App`(所有客户端聚合体)、`New`、`NewOption`(加载 config.yaml)、`Hunt`(启动+优雅关闭)、`Go`(安全 goroutine) |
| `sharklog` | 双通道日志(控制台+Kafka) | `SharkLog`、`New`、`SetKafkaWriter` |
| `sharkdb` | MySQL GORM 封装 | `NewDb`、`SharkTable`(链式查询)、`TableScan`(Keyset 游标分页) |
| `sharksql` | SQL 条件构建器 + 40+ 函数 | `SqlBuilder`、`NewSql`、`PageQuery[T]`、`IsDuplicateKey` |
| `sharkredis` | Redis 客户端(自动探测 cluster/client) | `NewCluster`、`NewClient`、`Helper`、`ScanKeys`、`DeleteKeys` |
| `sharkkafka` | Kafka 生产/批量消费 | `SharkKafka`、`Writer`、`BatchConsumer` |
| `sharkgrpc` | gRPC 服务发现(基于 Redis) | `RpcServer`、`New`、`GetRpcConnection` |
| `sharkhttp` | Gin HTTP 封装(3 个内置中间件) | `New`、`WsUpgrader`、中间件可单独导出 |
| `sharktimer` | Redis ZSet 轻量级定时器(±1s) | `Timer`、`AddTimer`、`RemoveTimer`、`DefaultCallback` |
| `sharksnowflake` | Snowflake ID(int64) | `NewSnowflake`、`Generate` |
| `sharkdecimal` | 高精度数值(decimal 封装) | `Normalize[N|2|6]` |
| `sharkelastic` | ES 客户端 | `SharkElastic`、`CreateIndex`、`Search`、`Insert` |
| `sharkcache` | 多层缓存防击穿 | `Cache[T]`、`New`、`Get`(singleflight+seeker链) |
| `sharkauth` | RBAC 权限树(多叉树) | `AuthNode`、`NormalizeAuthTree`、`PruneAuth`、`SyncAuthTree`、`Permissions`、`Flatten` |
| `sharkerror` | 统一业务错误 | `New`(code)、`WithData`、`WithErr`、`WithMsg` |
| `sharkfunc` | 泛型/并发工具 | `WithTimeout`、`ParallelCall`、`DrainChannelN`、`Recover`、`Pointer[T]` |
| `sharkutils` | 通用工具 | `RandNum`、`Md5`、`GetClientIp`、`BcryptHash/Check` |
| `sharkjson` | JSON 序列化 | `ParseJsonBytes[T]`、`ToJsonString` |
| `sharkverify` | TOTP 两步验证 | `NewSecret`、`VerifyCode`、`GetQrCodeUrl` |
| `sharkzip` | Zlib 压缩 | `Compress`、`Decompress` |
| `sharkrabbitmq` | RabbitMQ(批量消费) | `Client`、`Publish`、`Consume`、`BatchConsume` |
| `sharkmongodb` | MongoDB | `New` (mongo.Client) |
| `sharkminio` | MinIO 对象存储 | `New` (minio.Client) |
| `sharketcd` | etcd | `New` (clientv3.Client) |
| `sharkrisingwave` | RisingWave 流数据库 | `New` (*gorm.DB) |
| `sharkeswhere` | ES Query DSL 构建 | 条件构建 |
| `sharkbufferpool` | 字节缓冲区复用 | 内部性能优化 |

## App 初始化流程(16步,顺序固定)

```
1. 创建 context/WaitGroup
2. 初始化日志(sharklog)
3. Kafka 连接(dev/test 环境自动将日志写入 Kafka topic: "{project}_game_log")
4. Redis 连接(先试 Cluster, 失败回退 Client)
5. 独立配置的 Redis Cluster/Client(覆盖自动探测)
6. MySQL GORM(sharkdb.NewDb)
7. Elasticsearch(sharkelastic.New)
8. RabbitMQ(CRC16 哈希选择 broker 节点)
9. RisingWave(sharkrisingwave.New)
10. etcd(sharketcd.New)
11. MongoDB(sharkmongodb.New)
12. MinIO(sharkminio.New)
13. 定时器(依赖 Redis)
14. gRPC 服务(依赖 Redis 做服务发现)
15. HTTP 服务(Gin)
16. 异步启动 pprof/health 服务
```

> 各中间件仅为 nil/非 nil 判断，无配置不初始化。业务代码直接用 `app.Db`、`app.RedisClient` 等。

## config.yaml 配置

```yaml
http: 3928
grpc: 3929
health: 3930
pprof: 3931
env: dev          # dev/test/prod
id: "1"
timer: true

db:     { host, user, password, database, max_idle_conns, max_open_conns, ... }
redis:  { host[], password, replace_from, replace_to, pool_size, min_idle_conns }
kafka:  { host[], user, password, tls }
elastic:{ host[], user, password }
mongodb:{ host, user, password }
rabbitmq:{ host[], user, password }
minio:  { host, user, password }
risingwave:{ host, user, password, database }
etcd:   { host[], user, password }
redis_cluster: { host[], ... }  # 明确指定集群
redis_client:  { host[], ... }  # 明确指定单机
```

**配置优先级:** 环境变量(REDIS_HOST) > config.yaml > 代码默认值。密码支持 RSA 解密(`SHARK_PRIVATE_KEY` 环境变量)。

## 优雅关闭(Hunt 流程)

1. 调用所有 `AppComponent.Start()`
2. 启动 gRPC(`Grpc.Run()`)
3. 阻塞等待 SIGTERM/SIGINT
4. → `cancelFunc()` 取消 context
5. → sleep 500ms(等待请求完成)
6. → `Wg.Wait()`(等待 goroutine)
7. → 关闭日志(Kafka Writer)

## 核心功能速查

### 数据库查询

```go
// SharkTable 链式查询
sharkdb.NewTable(db.Table("users")).Eq("status",1).Gte("age",18).Desc("id").Gorm().Find(&users)

// SqlBuilder 构建 WHERE
sharksql.NewSql().Eq("status","pending").Or(sharksql.NewSql().Eq("status","done"))
sharksql.NewSql().EqCol("u.id","o.user_id")  // 字段对字段

// Keyset 游标分页(深分页性能不衰减)
scan := sharkdb.NewTableScan[User]().PageSize(500).Asc("create_time")
scan.Next(db.Where(...), lastCursor)   // 下一页
scan.Prev(db.Where(...), firstCursor)  // 上一页
scan.Export(ctx, db, "标题", headers, rowFn) // Excel 导出
```

### Redis

```go
sharkredis.NewCluster(ctx, config)  // 集群模式
sharkredis.NewClient(ctx, config)   // 单机模式
sharkredis.ScanKeys(ctx, client, "pattern:*")  // 流式 scan
sharkredis.DeleteKeys(ctx, client, "pattern:*") // 批量删除
```

### HTTP 中间件(3个内置,按序执行)

1. `recoveryMiddleware` — panic 恢复(记录调用栈+请求体+路径)
2. `corsMiddleware` — 允许所有 Origin, 支持 GET/POST
3. `errorMiddleware` — `sharkerror.Error` 转 JSON(code+msg+data)

### gRPC 服务发现

```go
// 服务端: sharkgrpc.New(ctx, project, rdb, logger, port)
// 客户端: server.GetRpcConnection("service-name") → *grpc.ClientConn
// 基于 Redis, round_robin 负载均衡, 指数退避重试
```

### 缓存防击穿

```go
sharkcache.New[User](localGetter, redisGetter, dbGetter).Get(userId)
// singleflight 保证同一 key 只查一次, seeker 链逐级回退
```

### RBAC 权限树

```
NormalizeAuthTree(fullTree, depth) → 初始化(默认无权限)
PruneAuth(tree)                     → 裁剪空节点
SyncAuthTree(full, child)           → 前端编辑树
Permissions(tree)                   → URL→权限路径映射(O(1)鉴权)
Flatten(tree)                       → {"路径":权限值}(Redis存储)
```

### 定时器

```go
sharktimer.NewTimer(ctx, project, name, id, rdb)
timer.AddTimer(30min, callback)     → 延迟执行
timer.AddTimeWithId("id", 30min, cb) → 业务 ID 关联
timer.RemoveTimer(id)               → 提前取消
timer.DefaultCallback(fn)           → 默认回调(未注册的回调)
```

### Snowflake

```go
sf := sharksnowflake.NewSnowflake()  // 41位时间戳+19位序列号
sf.Generate()  // int64, 52万/秒
```

### 高精度数值

```go
sharkdecimal.Normalize2(19.999)   // → 19.99(Round+Truncate)
sharkdecimal.Normalize6(7.12345678) // → 7.123456
sharkdecimal.Normalize(v, precision)
```

## 关键依赖速查

| 依赖 | 用途 |
|---|---|
| gin-gonic/gin | HTTP |
| gorm.io/gorm | ORM |
| go.uber.org/zap | 日志 |
| redis/go-redis | Redis |
| segmentio/kafka-go | Kafka |
| shopspring/decimal | 高精度数值 |
| spf13/viper | 配置加载 |
| panjf2000/ants | goroutine 池(定时器) |
| golang.org/x/sync/singleflight | 缓存防击穿 |
| pquerna/otp | TOTP 验证 |
| xuri/excelize | Excel 导出(TableScan) |
| gorilla/websocket | WebSocket |

## 项目入口(main.go)关键信息

- 项目名: `kgame`
- 实例名: `game-test`
- Swagger 注解: `@title game-demo API`, `@BasePath /api`, `ApiKeyAuth(x-token)`
- 端口配置(默认): http=3928, grpc=3929, health=3930, pprof=3931