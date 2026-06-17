# Shark

一个轻量级、模块化的 Go 微服务开发框架，内置了十余种基础设施组件的连接管理和实用工具。

## 项目简介

Shark 封装了微服务开发中常见的中间件和工具库，提供统一的配置加载、连接管理和最佳实践。各子包均可独立使用，无需依赖完整的框架上下文。

**模块路径：** `github.com/lornshark/shark`

**Go 版本要求：** 1.26+

## 特性

- **统一的配置管理** — 所有组件通过 `config.yaml` 统一配置，支持 json/yaml/mapstructure 标签
- **双通道日志** — 控制台（Console Encoder）+ Kafka（JSON Encoder），支持结构化日志和优雅退出
- **内置服务发现** — gRPC 基于 Redis 的服务注册与发现，支持动态地址更新和 round_robin 负载均衡
- **高性能 ID 生成** — 改进型 Snowflake 算法，每秒 52 万个 ID，int64 类型前端安全
- **SQL 条件构建器** — 流式 API 构建参数化 WHERE 子句，防 SQL 注入，支持 AND/OR 嵌套
- **Keyset 游标分页** — 泛型表扫描器，深分页性能不受数据量影响，支持 Excel 流式导出
- **高精度数值** — 基于 shopspring/decimal 的类型归一化和精度控制，消除浮点累积误差
- **批量消费者** — Kafka 批量拉取 + 管道缓冲 + 自动提交 offset，支持重试和优雅停止
- **定时器** — Redis ZSet 轻量级定时器，误差 ±1s，协程池异步执行回调
- **泛型工具** — PageQuery 分页、TableScan 扫描、WithTimeout 超时控制等泛型函数

## 快速开始

### 安装

```bash
go get github.com/lornshark/shark
```

### 配置文件

在项目根目录创建 `config/config.yaml`：

```yaml
http: 3928
grpcx: 3929
health: 3930
pprof: 3931

db:
  host: 127.0.0.1:4000
  user: root
  password: yourpassword
  database: mydb

redis:
  host: 127.0.0.1:6379
  password: ""

kafka:
  host: 127.0.0.1:9092
  user: ""
  password: ""
```

### 启动应用

```go
package main

import (
    "github.com/lornshark/shark/sharkapp"
)

func main() {
    options := sharkapp.NewOption("myproject", "instance-1")
    app, err := sharkapp.New(options)
    if err != nil {
        panic(err)
    }
    // 注册业务模块
    app.Hunt(&MyService{svc: app})
}
```

## 包结构

| 包名 | 功能 | 核心导出 |
|------|------|----------|
| `sharkapp` | 应用启动与依赖注入 | `App`, `New`, `NewOption`, `Hunt` |
| `sharklog` | 双通道日志系统 | `SharkLog`, `New`, `SetKafkaWriter` |
| `sharkdb` | 数据库连接与工具 | `sharktable`, `TableScan`, `PageQuery` |
| `sharkredis` | Redis 客户端 | `New`, `Lock`/`Unlock` 分布式锁 |
| `sharkkafka` | Kafka 生产消费 | `SharkKafka`, `Writer`, `BatchConsumer` |
| `sharkgrpc` | gRPC 服务发现 | `RpcServer`, `New`, `GetRpcConnection` |
| `sharkhttp` | HTTP 服务 | `New`, 中间件 |
| `sharktimer` | 定时器 | `Timer`, `AddTimer`, `RemoveTimer` |
| `sharksnowflake` | Snowflake ID 生成 | `Snowflake`, `Generate` |
| `sharksql` | SQL 条件构建 | `Builder`, `PageQuery`, `Eq`, `In`, `Like` 等 |
| `sharkdecimal` | 高精度数值 | `Normalize`, `Normalize2`, `Normalize6` |
| `sharkelastic` | Elasticsearch | 搜索接口封装 |
| `sharketcd` | etcd 键值存储 | `Put`, `Get`, `GetPrefix` |
| `sharkmongodb` | MongoDB 文档数据库 | CRUD 操作 |
| `sharkminio` | MinIO 对象存储 | 文件上传/下载 |
| `sharkrabbitmq` | RabbitMQ 消息队列 | 发布/消费 |
| `sharkrisingwave` | RisingWave 流式数据库 | GORM + zap 日志适配器 |
| `sharkerror` | 错误处理 | 统一错误码 |
| `sharkjson` | JSON 工具 | `ToJsonString`, gjson 封装 |
| `sharkcache` | 缓存抽象 | 本地/Redis 缓存 |
| `sharkauth` | 认证鉴权 | Token 验证 |
| `sharkfunc` | 通用函数 | `DrainChannelN`, `WithTimeout` |
| `sharkutils` | 工具函数 | 字符串/时间/加密 |
| `sharkverify` | 验证码/OTP | TOTP 生成与验证 |
| `sharkzip` | ZIP 压缩 | 打包/解压 |

## 核心模块详解

### 日志系统 (`sharklog`)

双通道日志输出：控制台（Console Encoder）+ Kafka（JSON Encoder），通过 `zapcore.NewTee` 合并。

```go
log := sharklog.New(ctx, "myproject", "instance-1")

// 可选：设置 Kafka Writer 启用远程日志推送
log.SetKafkaWriter(kafkaWriter)

// 使用 zap 原生方式记录日志
log.Zap.Info("服务启动成功")
log.Zap.Error("数据库连接失败", zap.Error(err))
log.Zap.Info("订单创建", zap.String("order_id", "ORD-001"), zap.Float64("amount", 99.99))
```

### 数据库（`sharkdb`）

#### TableScan — Keyset 游标分页

完美替代 OFFSET/LIMIT 深分页，性能不受数据量影响。

```go
scan := sharkdb.NewTableScan[User]().
    PageSize(500).
    Asc("create_time", "id")

var last *User
for {
    results, _ := scan.Next(db.Where("deleted = 0"), last)
    if len(results) == 0 {
        break
    }
    last = &results[len(results)-1]
    for _, u := range results {
        fmt.Println(u.Id, u.Name)
    }
}
```

#### Excel 导出

基于 TableScan 的流式 Excel 导出，内存占用仅为一页数据量。

```go
filePath, err := scan.Export(
    ctx, db.Where("status = 1"),
    "用户列表",
    []any{"ID", "姓名", "创建时间"},
    func(u User) []any {
        return []any{u.Id, u.Name, u.CreateTime.Format("2006-01-02 15:04:05")}
    },
)
```

### SQL 条件构建器 (`sharksql`)

#### Builder — 动态 WHERE 子句

流式 API，空值自动跳过，防 SQL 注入，支持 AND/OR 嵌套。

```go
// AND 查询（同一 group）
b := sharksql.NewBuilder().
    Eq("status", 1).
    Gte("age", 18).
    Like("name", "张")

// OR 查询（不同 group）
b := sharksql.NewBuilder().
    Eq("created_by", uid).
    Or(sharksql.NewBuilder().Eq("assignee", uid))

// 复杂嵌套：(deleted = 0) AND (status = pending OR status = in_progress) AND (created_by = uid OR assignee = uid)
b := sharksql.NewBuilder().Eq("deleted", 0).
    And(sharksql.NewBuilder().Eq("status", "pending").Or(sharksql.NewBuilder().Eq("status", "in_progress"))).
    And(sharksql.NewBuilder().Eq("created_by", uid).Or(sharksql.NewBuilder().Eq("assignee", uid)))

sql, args := b.Build()
db.Where(sql, args...).Find(&results)
```

#### PageQuery 泛型分页

```go
users, total, err := sharksql.PageQuery[User](db.Where("status = 1"), 1, 20)
```

### gRPC 服务发现 (`sharkgrpc`)

基于 Redis 的服务注册与发现，支持动态地址更新和 round_robin 负载均衡。

```go
// 服务端
server := sharkgrpc.New(ctx, "myproject", rdb, logger, 50051)
pb.RegisterEchoServer(server.Server, &EchoServiceImpl{})

// 客户端
conn, _ := server.GetRpcConnection("user-service")
client := pb.NewUserServiceClient(conn)
resp, _ := client.CreateOrder(ctx, &pb.CreateOrderReq{UserId: 12345})
```

### 高精度数值 (`sharkdecimal`)

类型归一化 + Round(高精度) + Truncate(截断)，消除浮点累积误差。

```go
// 金额（2 位小数）
price := sharkdecimal.Normalize2(19.999)   // → 19.99
tax := sharkdecimal.Normalize2("3.50")     // → 3.50
total := price.Add(tax)                    // → 23.49

// 汇率（6 位小数）
rate := sharkdecimal.Normalize6(7.12345678) // → 7.123456
cnyAmount := sharkdecimal.Normalize2(usdAmount.Mul(rate))

// 自定义精度
pi := sharkdecimal.Normalize(3.1415926535, 4) // → 3.1415
```

### Snowflake ID (`sharksnowflake`)

改进型 Snowflake 算法，int64 类型，JavaScript 前端可直接安全使用。

```go
sf := sharksnowflake.NewSnowflake()

// 生成单个 ID
id := sf.Generate() // → 78140312576000

// 批量生成（预填充 Redis 列表）
ids := make([]int64, 1000)
for i := range ids {
    ids[i] = sf.Generate()
}
```

### Kafka (`sharkkafka`)

#### 生产者

```go
sk, _ := sharkkafka.New(ctx, cfg, logger)
defer sk.Close()

writer, _ := sk.Writer("order-events")
writer.WriteMessages(ctx, kafka.Message{
    Key:   []byte("order-12345"),
    Value: []byte(`{"status":"created"}`),
})
```

#### 批量消费者

```go
sk.BatchConsumer("order-events", "order-processor", func(msgs []kafka.Message) bool {
    for _, msg := range msgs {
        var order Order
        json.Unmarshal(msg.Value, &order)
        processOrder(order)
    }
    return true // 返回 false 停止消费
})
```

### 定时器 (`sharktimer`)

基于 Redis ZSet 的轻量级单机定时器，误差 ±1s。

```go
timer := sharktimer.NewTimer(ctx, "myproject", "order-timer", "inst-1", rdb)

// 30 分钟后取消订单
timerId := timer.AddTimer(30*time.Minute, func() {
    fmt.Println("订单超时，自动取消")
    orderService.Cancel("12345")
})

// 在触发前取消
timer.RemoveTimer(timerId)
```

## 作者

Lornshark

## 许可证

MIT License