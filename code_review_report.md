# Shark 项目代码评审报告

## 一、项目概述

Shark 是一个 Go 语言编写的微服务基础设施框架，统一封装了 MySQL(TiDB)、Redis、Kafka、Elasticsearch、MongoDB、RabbitMQ、MinIO、etcd、RisingWave 等常用中间件的连接管理和配置加载。项目结构清晰，采用"应用聚集模式"（App 结构体聚合所有中间件客户端）。

---

## 二、模块逐项评审

### 1. `main.go` — 入口文件

**问题：**

- 🔴 **严重**：`main.go` 中包含大量注释掉的代码（第 38-46 行、49-67 行），包括 `Serialize`, `ColumnAs`, `LeftJoin`、`OnEq` 等辅助方法。这些方法已有正式实现（在 `sharksql` 包中），但残留代码会造成混淆，且 `Serialize` 常量未定义直接使用会导致编译错误。
- 🟡 **中**：`app.Hunt(&Test{svc: app})` 直接将 `*App` 赋值给 `Test`，但 `Test.Start()` 没有任何实质性逻辑。作为示例代码可以保留，但建议添加注释说明这是示例模板。
- 🟢 **低**：Swagger 注释中的 `@BasePath` 应写为 `@BasePath`（实际是 `@BasePath`），当前缺少了 `h` 字母。

**建议：** 清理注释代码，将 `Serialize`、`ColumnAs` 等已弃用代码移除。Swagger 路径修正为 `/api`。

---

### 2. `config/config.yaml` — 配置文件

**问题：**

- 🔴 **严重**：配置文件中明文存储了生产环境密码（`CEki57pxTJyYaLD`），多处复用同一密码。这是严重的安全隐患。配置文件中的密码应该：
  - 生产环境通过环境变量注入
  - 或使用密钥管理服务（如 Vault）
  - 测试环境密码也不应提交到 Git 仓库
- 🟡 **中**：YAML 配置文件路径硬编码在 `Option/ReadInConfig` 中（只查找 `./config.yaml` 和 `./config/config.yaml`），灵活性不足。

**建议：** 将所有密码替换为环境变量占位符 `${DB_PASSWORD}`、`${REDIS_PASSWORD}` 等，在代码中通过 `os.Getenv()` 或 viper 的 `AutomaticEnv()` 读取。

---

### 3. `sharkapp/option.go` — 配置加载

**问题：**

- 🔴 **严重**：`read_slices` 方法定义在 `Options` 上但使用了值接收者 `(o *Options)`，而该方法不修改任何 `Options` 字段——它应该是一个独立函数而非方法。
- 🟡 **中**：`ReadInConfig()` 失败时如果是非 `ConfigFileNotFoundError` 错误调用 `panic(err)`。直接 panic 不够友好，建议返回 error 让调用方决定如何处理。
- 🟡 **中**：配置中多个中间件的 host 通过 `read_slices` 解析后，很多只取 `hosts[0]`（如 db, minio, mongodb, risingwave），但没有对 `len(hosts) == 0` 的情况做额外处理（只在 `len(hosts) > 0` 时创建配置，跳过时没问题，但取 `hosts[0]` 时已确保了非空）。
- 🟢 **低**：`NewOption` 函数太长（~200 行），可将各中间件的配置解析抽取为独立的 parse 函数。
- 🟢 **低**：`WithXxx` 方法未在代码中实现（从文档注释来看有 `WithDB`、`WithRedis` 等方法，但 option.go 中未找到定义，可能在其他文件或尚未实现）。

**建议：** 将 `read_slices` 改为包级函数；`panic` 改为返回 error；拆分 `NewOption` 为小的 parse 函数。

---

### 4. `sharkapp/sharkapp.go` — 应用核心框架

**问题：**

- 🔴 **严重**：`New()` 函数对每个中间件的连接都使用相同的错误处理模式：`app.Logger.Error(...)` 然后 `return nil, err`。这种模式导致：
  - 任何单个中间件连接失败都会阻止整个应用启动（All-or-nothing 策略）。在生产环境中，某些中间件（如日志用 Kafka）不应该是 critical path。
  - 建议引入"软失败"选项，允许非核心中间件连接失败后降级运行。
- 🟡 **中**：Kafka 初始化成功后重复判断 `if options.kafka != nil`（第 176 行和第 190 行），第二处判断是冗余的——如果 kafka 初始化成功，`app.Kafka` 必然不为 nil，但此处目的是为了打日志，可以用 `if app.Kafka != nil` 替代，减少对 options 的依赖。
- 🟡 **中**：Redis 集群模式回退到单机模式的逻辑（第 198-217 行）依赖字符串匹配 `"cluster support disabled"`，这是脆弱的错误处理方式。go-redis 库可能在不同版本中改变错误消息，建议使用错误类型判断。
- 🟡 **中**：RabbitMQ 的 broker 选择使用 CRC16 哈希取模（第 274-275 行），但实际上 `hosts` 从 YAML 解析时可能已经是单元素数组。CRC16 取模方案可以工作，但注释不够清晰。
- 🟡 **中**：`Hunt()` 方法中使用 `time.Sleep(100ms)` 等待 goroutine 就绪（第 448 行）。这是"magic sleep"反模式——goroutine 启动时机不确定，100ms 不能保证所有 goroutine 都就绪，建议使用 channel 通知或其他同步机制。
- 🟡 **中**：`corsMiddleare()` 函数名有拼写错误：应为 `corsMiddleware`（少了一个 `w`）。
- 🟡 **中**：`Hunt()` 方法中同时监听 `SIGTERM` 和 `SIGINT`，但调用了两次 `signal.Notify` 向同一个 channel——标准的 Go 写法是一次调用传递多个信号：`signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)`。
- 🟢 **低**：`health_service` 中设置 `Content-Type` header 在 `WriteHeader` 之后——必须先设置 header 再调用 `WriteHeader`。当前代码虽然因为 `WriteHeader(200)` 后 `Header().Set()` 会被忽略，但 `Write([]byte(...))` 会隐式设置 Content-Type，实际运行可能正常工作，但顺序不符合最佳实践。
- 🟢 **低**：`pprof` 和 `health_service` 都使用了 `http.ListenAndServe`，但 HTTP 服务使用了 Gin 的 `router.Run()`。风格不统一。同时 `health_service` 使用了 `http.HandleFunc` 的默认 mux，与 pprof 使用的独立 mux 不一致。

**建议：**
- 引入中间件"软失败"模式
- 修复 `corsMiddleare` → `corsMiddleware`
- 合并信号监听
- 修复 header 设置顺序
- 去掉 magic sleep

---

### 5. `sharkdb/sharkdb.go` — 数据库连接

**问题：**

- 🟡 **中**：`NewDb()` 中 DSN 构建使用了 `fmt.Sprintf(dsn, ...)`，其中 `dsn` 本身也是一个格式化字符串。如果密码中包含 `%` 字符，会导致格式化错误。应直接字符串拼接或使用安全的 URL 编码。
- 🟡 **中**：连接池参数（MaxIdleConns=20, MaxOpenConns=100）是硬编码的。不同业务场景对连接数的需求不同，建议通过 Config 结构体开放自定义。
- 🟡 **中**：TLS 配置中强行使用 `ServerName: config.Host`，但 Host 格式包含端口（如 `192.168.191.100:4000`），作为 ServerName 传递给 TLS 可能不合法（TLS ServerName 不应包含端口）。
- 🟢 **低**：`LogMode` 方法创建了新的 `log` 实例但未复制 `level` 字段——在调用 `LogMode` 之前 `z.level` 为零值。这是正确的行为（创建新实例时继承 logger 和新 level），但可能造成误解。

**建议**：
- DSN 构建改用 `net/url` 编码
- 连接池参数可配置化
- TLS ServerName 去除端口号

---

### 6. `sharkdb/sharktable.go` + `sharkdb/tablescan.go` — 表操作封装

**问题：**

- 🟡 **中**：`isEmpty()` 方法对指针类型（`reflect.Ptr`）递归调用自身，但对于 `**int` 这样的多级指针会无限递归（每层都是 Ptr Kind）。虽然实际业务中不太可能出现多级指针，但防御性编程应限制递归深度或使用循环解引用。
- 🟡 **中**：`Like`、`NotLike`、`LikeLeft`、`LikeRight` 方法中有重复的指针解引用逻辑。可以抽取为一个公共方法。
- 🟡 **中**：`SelectWithTiflash` 使用 `t.db.Statement.Table` 动态拼接表名到 SQL 注释中。如果表名包含特殊字符，可能存在风险。
- 🟢 **低**：`JsonSearchOne` 方法名拼写错误：应为 `JsonSearchOne` → `JSONSearchOne`（已正确）。

**建议**：抽取公共指针解引用方法；限制 `isEmpty` 递归深度。

---

### 7. `sharksql/sharksql.go` — SQL 构建器 (1715 行)

**问题：**

- 🔴 **严重**：文件过长（1715 行），包含了 Where 条件构建、聚合函数、JSON 操作、分页查询、UPDATE 构建等多种职责，严重违反单一职责原则。建议拆分为多个文件：
  - `condition.go` — 条件函数（Eq, Gt, Like 等）
  - `aggregate.go` — 聚合函数（Sum, Count, Avg 等）
  - `json.go` — JSON 操作函数
  - `pagination.go` — 分页查询
  - `update.go` — ToUpdate 相关
- 🟡 **中**：`Where()` 函数（1398 行起）使用了 `sql` 结构体 tag 来反射生成 WHERE 条件。这是一种隐式约定，tag 的错误（如拼写错误）在编译期无法检测，只能在运行时发现条件不对。
- 🟡 **中**：`buildCondition()` 函数判断 LIKE/NOT LIKE/LIKEL/LIKER 等使用了大小写不敏感的字符串匹配（`strings.ToUpper`），但原始 sql tag 大小写混合。建议统一为全大写标记以避免歧义。`"NOT LIKEL ?"` 和 `"NOT LIKER ?"` 的命名不够直观。
- 🟡 **中**：`ToUpdate()` 函数中 `column` 从 `json` tag 读取（第 1653 行），但 `update` 操作应该使用数据库列名而非 JSON 字段名——这两者通常不同（如 JSON 用 `pageSize`，数据库用 `page_size`）。当前设计假设 JSON 字段名与数据库列名相同，这不一定成立。
- 🟡 **中**：`Where()` 函数的参数名拼写错误：`LIKEL` 应为 `LikeLeft` 的缩写，但 `LIKER` 应为 `LikeRight`，当前含义反了（`LIKEL` 是左匹配后缀，`LIKER` 是右匹配前缀）。但代码实现是正确的：`LIKEL` → `value%`、`LIKER` → `%value`。
- 🟢 **低**：`isDecimal()` 和 `isDecimalType()` 两个函数中，`isDecimalType` 通过 `reflect.New(t).Elem()` 创建零值实例来判断类型，这在 hot path 中可能有性能开销。对于 `reflect.Struct` 类型的判断，`reflect.New` 涉及内存分配。
- 🟢 **低**：`Case` 和 `CaseAs` 函数中，`pairs` 参数使用 `...any` 类型，编译期无法保证参数数量为偶数。奇数个参数时最后一个作为 ELSE，但这是隐式约定。
- 🟢 **低**：`buildCondition` 中 `IN (?)` 的判断使用了大写比较 `strings.Contains(upper, "IN (?)")`, 且在 LIKE 之前判断——这是正确的，但如果有 `"column IN (?) AND name LIKE ?"` 这样的复杂 tag，当前逻辑会直接返回整个 tag 和 value，可能导致问题。不过当前设计预期每个 tag 只包含一个条件，所以问题不大。
- 🟢 **低**：`IndexIgnoreCase` 函数内部对 `s` 假设已经是大写，但又对 `substr` 做了 `strings.ToUpper`——实际上 `s` 可能不是大写的。函数注释说"参数 s 必须已经是大写形式"，但调用方 `buildCondition` 确实传入了 `upper`。

**建议**：拆分文件；为 sql tag 添加强校验；统一 LIKE 变体命名。

---

### 8. `sharksql/builder.go` — SQL Builder

**问题：**

- 🟡 **中**：`And()` 方法（第 680-702 行）合并单 group 的 builder 时，直接将条件追加到当前 group，但没有检查两个 group 之间的 args 索引是否冲突。这在当前设计下不会出问题（因为 args 是线性追加的），但如果有嵌套的 AND 组合场景可能产生混淆。
- 🟡 **中**：`Build()` 方法中，单个 group 只有一个条件时不加括号，但多个 group 时每个 group 的条件数 > 1 时会加括号。这可能导致 SQL 逻辑与期望不一致。例如：`group1: [a=1]`, `group2: [b=2, c=3]` 生成 `a=1 OR (b=2 AND c=3)`，而不是 `(a=1) OR (b=2 AND c=3)`。虽然语义等价，但加上一致的括号更清晰。
- 🟢 **低**：`isEmpty()` 和 `isSlice()` 都使用 `reflect` 包，每次调用都有一定开销。对于热路径，可以考虑类型断言优化。
- 🟢 **低**：`Build()` 方法没有对生成的 SQL 做长度限制，极端情况下可能生成超长 SQL。

**建议**：统一括号生成逻辑；考虑 SQL 长度限制。

---

### 9. `sharkerror/sharkerror.go` — 错误处理

**问题：**

- 🟢 **低**：`Error.Error()` 方法返回 `"code=xxx msg=xxx"` 格式，但 Data 字段的信息完全丢失。如果用于调试，建议包含 Data 信息。
- 🟢 **低**：`WithErr()` 方法将原始错误的 `Error()` 文本赋值给 `Data`，丢失了原始错误类型。如果上游需要通过 `errors.Is/As` 判断原始错误，会失败。考虑使用 `fmt.Errorf("%w", err)` 模式或保留原始错误类型。

**建议**：`Error()` 方法可包含 Data；`WithErr` 考虑错误链。

---

### 10. `sharkredis/sharkredis.go` + `helper.go` — Redis 模块

**问题：**

- 🔴 **严重**：`NewClient()` 方法（第 126 行）直接使用 `config.Host[0]` 而没有检查 `len(config.Host) > 0`。如果 Host 为空切片，将导致 index out of range panic。
- 🟡 **中**：`ScanKeys()` 返回的 `chan error` 缓冲区大小为 1，但可能有多个 goroutine 同时尝试发送错误。第 54-57 行的 select 使用了 `default` 分支来避免阻塞——这意味着除了第一个错误外，其他错误将被静默丢弃。
- 🟡 **中**：`DeleteKeys()` 使用 batch delete（每批 50），但没有使用 Redis Pipeline 或 UNLINK（异步删除）。大量 key 删除时使用同步 `DEL` 可能阻塞 Redis。
- 🟢 **低**：`ScanKeys()` 中 `ForEachMaster` 的返回错误处理后（第 75-79 行）直接关闭了输出 channel 并 return，但此时可能已经有 goroutine 在运行（wg.Add 在 ForEachMaster 的回调中）。这会导致 goroutine 泄漏。应该在 return 前确保没有 goroutine 在运行。

**建议**：
- `NewClient` 检查 Host 长度
- `ScanKeys` 增加错误 channel 缓冲区
- `DeleteKeys` 使用 UNLINK
- 修复 `ScanKeys` 潜在的 goroutine 泄漏

---

### 11. `sharklog/sharklog.go` — 日志模块

**问题：**

- 🟡 **中**：`logWriter.Write()` 中使用 `context.Background()` 向 Kafka 发送消息（第 222 行）。这意味着即使上层 context 已取消，日志仍然尝试写入。这在优雅关闭期间是刻意设计（最后一条日志需要成功发送），但如果 Kafka 不可用，会阻塞直到 Kafka 超时。建议增加超时控制。
- 🟡 **中**：优雅退出检测（第 229-234 行）通过字符串包含 `"****************server exit****************"` 来判断是否需要关闭 Kafka Writer。这是一种隐式约定——如果日志消息恰好包含这个字符串（虽然不太可能），会错误触发关闭。
- 🟢 **低**：每次调用 `Write()` 都生成一个新的 Snowflake ID（`w.snowFlake.Generate()`）。高频日志场景下，Snowflake ID 生成有一定开销。但每条日志有唯一 ID 是可追溯性的好设计。
- 🟢 **低**：`New` 函数中初始化了 Snowflake（第 106 行），但没有传入 worker ID 和 datacenter ID，可能导致不同实例生成重复的 Snowflake ID。

**建议**：Kafka 写入增加超时；退出检测使用更明确的信号（如专门的关闭方法而非字符串匹配）。

---

### 12. `sharkhttp/sharkhttp.go` + `middleare.go` — HTTP 模块

**问题：**

- 🔴 **严重**：文件名拼写错误：`middleare.go` 应为 `middleware.go`（少了一个 `w`）。
- 🟡 **中**：`corsMiddleare()` 函数名也拼写错误（应为 `corsMiddleware`），且 Return 类型是 `gin.HandlerFunc` 但注释中写的是 `sharkhttp.CorsMiddleware()`（应该用实际函数名）。
- 🟡 **中**：`recoveryMiddleware` 中先读取了 `GetRawData()` 再放回 `Body`。这在 Gin 中是标准做法，但每次请求都会读取完整请求体（即使没有 panic），对性能有影响。可以考虑延迟读取或只记录大小。
- 🟡 **中**：`errorMiddleware` 将所有错误以 HTTP 200 返回。这是有意设计（业务错误不应使用 HTTP 状态码），但与 RESTful 标准不一致。建议在文档中明确说明。
- 🟢 **低**：`wsUpgrader` 变量定义了但似乎未在代码中被使用（检查 `sharkhttp` 包中是否有其他文件引用）。
- 🟢 **低**：`router.Run()` 没有处理返回的 error（第 73 行）。如果端口被占用，错误会被静默丢弃。

**建议**：
- 重命名 `middleare.go` → `middleware.go`
- 修复函数名拼写
- gin.Run() 返回的 error 应记录日志

---

### 13. `sharkauth/sharkauth.go` — 权限模块

**问题：**

- 🟡 **中**：`SyncAuthTree()` 直接修改传入的 `parent` 参数（第 538 行 `return parent`），文档中也明确说明了这一行为。但从函数签名看是返回 `[]*AuthNode`，调用方可能误以为是纯函数（无副作用）。建议在函数名上体现副作用，如 `MarkAuthTree` 或 `SyncAuthTreeInPlace`。
- 🟡 **中**：`PruneUnauthorizedAuthTree` 和 `SyncAuthTree` 内部各自定义了一个 `find` 函数，逻辑完全相同但无法共享（因为闭包捕获了不同的外部变量 `parent`/`child`）。可以抽取为独立的包级函数。
- 🟡 **中**：`Permissions()` 函数中同一个 URL 可能映射到多个权限路径（如 `/api/user/list` 既属于"系统管理.用户管理.用户列表"，又可能属于其他路径）。这是有意设计，但可能导致鉴权逻辑复杂化。
- 🟢 **低**：`NormalizeAuthTree` 中 `auth` 参数未做范围校验（只应有 1 或 2），但函数本身不会出错——只是可能产生意外结果。
- 🟢 **低**：`Flatten` 使用 `map[string]any` 作为 value 类型，但 value 始终为 1。建议使用 `map[string]struct{}` 或 `map[string]bool` 以明确语义。

**建议**：抽取公共 `find` 函数；Rename `SyncAuthTree` 或添加文档警告。

---

## 三、通用问题

### 3.1 安全问题
| 严重度 | 问题 | 位置 |
|--------|------|------|
| 🔴 | 明文密码存储在 YAML 配置中 | `config/config.yaml` |
| 🟡 | DSN 构建使用 fmt.Sprintf，密码特殊字符可能导致注入 | `sharkdb/sharkdb.go` |
| 🟡 | SQL tag 注入风险低但存在（列名由开发者控制） | `sharksql/sharksql.go` |

### 3.2 错误处理
| 严重度 | 问题 | 位置 |
|--------|------|------|
| 🔴 | `NewClient()` 未检查 Host 切片长度，可能 panic | `sharkredis/sharkredis.go:126` |
| 🟡 | 多处忽略错误返回值（如 `gin.Run()`、`sonic.Marshal()`） | 多处 |
| 🟡 | `ScanKeys` 仅缓冲 1 个错误，其他错误静默丢弃 | `sharkredis/helper.go:39` |

### 3.3 代码质量
| 严重度 | 问题 | 位置 |
|--------|------|------|
| 🟡 | `middleare.go` / `corsMiddleare` 拼写错误 | `sharkhttp/` |
| 🟡 | `sharksql.go` 1715 行，严重过长 | `sharksql/` |
| 🟡 | `Hunt()` 使用 magic sleep 等待 goroutine | `sharkapp/sharkapp.go:448` |
| 🟡 | 多处重复代码（Like 系列指针解引用、find 函数） | 多处 |
| 🟢 | 函数命名不一致：`NewDb` vs `NewCluster` vs `NewClient` | 多处 |

### 3.4 性能问题
| 严重度 | 问题 | 位置 |
|--------|------|------|
| 🟡 | `DeleteKeys()` 使用同步 DEL，应使用 UNLINK | `sharkredis/helper.go` |
| 🟡 | `isDecimalType()` 使用 reflect.New 创建零值实例 | `sharksql/sharksql.go:1502` |
| 🟢 | `recoveryMiddleware` 每次请求都读取完整 Body | `sharkhttp/middleare.go:92` |

### 3.5 设计与可维护性
| 严重度 | 问题 | 位置 |
|--------|------|------|
| 🔴 | 任何中间件连接失败即阻止启动（无"软失败"模式） | `sharkapp/sharkapp.go` |
| 🟡 | `sharksql.go` 混合了条件构建、聚合、JSON、分页等多种职责 | `sharksql/` |
| 🟡 | `Where` 函数使用隐式 sql tag，错误在编译期不可见 | `sharksql/sharksql.go` |
| 🟡 | 硬编码的连接池参数、超时时间不可配置 | 多处 |

---

## 四、测试覆盖

测试文件位于 `test/` 目录，包含各模块的单元测试和集成测试。但：
- 集成测试名称包含 `_integration_test.go` 但使用了 build tag 检查
- 暂未审查测试文件质量（需进一步分析）
- 建议检查 `TestXXX` 函数的覆盖率和边界条件

---

## 五、改进优先级建议

### 高优先级（应立即修复）
1. **密码安全**：从 `config.yaml` 中移除明文密码，改用环境变量
2. **panic 风险**：`NewClient` 检查 `Host` 切片长度
3. **goroutine 泄漏**：修复 `ScanKeys` 的 goroutine 管理
4. **所有或全无启动**：引入中间件"软失败"模式
5. **文件/函数命名拼写**：`middleare.go` → `middleware.go`

### 中优先级（建议在下一迭代修复）
1. DSN 安全构建（防止密码特殊字符问题）
2. `sharksql.go` 拆分文件
3. `Hunt()` 中去掉 magic sleep
4. 抽取重复代码（find, 指针解引用）
5. `DeleteKeys` 改用 UNLINK
6. Redis 回退逻辑改用错误类型而非字符串匹配

### 低优先级（可后续优化）
1. 连接池参数可配置化
2. 统一 `Error()` 包括 Data 信息
3. 日志 Kafka 写入增加超时
4. 代码长度规范化（`NewOption` 拆分）
5. `Flatten` 返回值类型优化

---

## 六、总结

Shark 框架整体设计思路清晰，对微服务开发中的常见中间件做了较好的统一封装。SQL 条件构建器和权限树模型是亮点，但存在一些工程实践方面的问题需要改进：

- **安全**：明文密码存储是最严重的问题
- **健壮性**：多处缺少边界检查、错误处理不完整
- **代码组织**：`sharksql.go` 过长，拼写错误
- **可配置性**：硬编码参数较多

整体评分：**7.5/10**

适合作为内部微服务开发框架使用，建议在上线前修复高优先级问题。