package sharkauth

import (
	"strings"
)

// AuthNode 权限树节点，表示权限体系中的一个节点。
// 权限树是一个多叉树结构，每个节点代表一个功能模块或API资源，
// 通过 Name 标识节点名称，通过 Children 构成层级父子关系。
//
// Auth 字段含义（三位状态码）：
//   - 0: 未设置/默认值，表示该节点的权限状态未确定（用于编辑态）
//   - 1: 拥有权限，表示用户/角色对该节点下的资源具有访问权限
//   - 2: 无权限，表示显示该节点但明确不具有访问权限
//
// 使用场景示例：
//   - 角色权限管理：为不同角色配置可访问的 URL 资源
//   - 父子角色继承：子角色权限需在父角色允许范围内裁剪
//   - 前端菜单渲染：根据 Auth 值决定菜单项的选中/禁用状态
type AuthNode struct {
	// Name 节点名称，通常是功能模块名称（如 "用户管理"、"订单管理"）
	// 在整个权限树中，Name 与其父节点路径组合构成唯一标识
	Name string `json:"name,omitempty"`

	// Children 子节点列表，构成权限树的层级结构
	// 父节点的权限范围包含所有子节点
	Children []*AuthNode `json:"children,omitempty"`

	// Urls 该节点关联的 API 接口路径列表
	// 一个权限节点可以对应多个 URL，URL 到权限路径的映射由 Permissions() 函数生成
	// 例如：["/api/user/list", "/api/user/detail"]
	Urls []string `json:"urls,omitempty"`

	// Auth 权限状态标志
	// 0: 未设置/默认值 1: 有权限 2: 无权限
	Auth int `json:"auth,omitempty"`
}

// NormalizeAuthTree 深拷贝并规范化权限树。
//
// 功能说明：
//  1. 对传入的权限树进行完全深拷贝，确保不修改原始数据
//  2. 清空所有节点的 Urls 字段（URL 信息在规范化中不需要保留）
//  3. 对于原始树中 Urls 不为空的节点，将其 Auth 设置为指定值
//
// 参数：
//   - nodes: 原始权限树根节点列表（允许多棵树并列）
//   - auth: 对所有有 URL 的节点统一设置的 Auth 值
//
// 返回值：
//   - 深拷贝并规范化后的全新权限树（与原树无共享引用）
//
// 使用场景：
//   - 创建角色的初始权限模板：将原始功能树拷贝一份，所有功能节点默认无权限
//   - 重置角色权限：基于完整功能树重新生成权限编辑界面
func NormalizeAuthTree(nodes []*AuthNode, auth int) []*AuthNode {
	// dfs 深度优先遍历克隆每个节点
	var dfs func(n *AuthNode) *AuthNode
	dfs = func(n *AuthNode) *AuthNode {
		if n == nil {
			return nil
		}
		// 创建新节点，仅拷贝 Name
		newNode := &AuthNode{
			Name: n.Name,
		}
		// 如果原节点关联了 URL，则设置 Auth 为传入值（表示该节点是"有实际功能"的节点）
		if len(n.Urls) > 0 {
			newNode.Auth = auth
		}
		// 清空 Urls：规范化后的树不需要 URL 信息
		newNode.Urls = nil
		// 递归克隆子节点
		if len(n.Children) > 0 {
			newNode.Children = make([]*AuthNode, 0, len(n.Children))
			for _, child := range n.Children {
				newNode.Children = append(newNode.Children, dfs(child))
			}
		}
		return newNode
	}
	// 遍历根节点数组并分别克隆
	res := make([]*AuthNode, 0, len(nodes))
	for _, n := range nodes {
		res = append(res, dfs(n))
	}
	return res
}

// PruneAuth 递归裁剪权限树，移除无效节点。
//
// 功能说明：
//  1. 对权限树进行完全深拷贝，不修改原数据
//  2. 从叶子节点开始自底向上递归裁剪
//  3. 删除同时满足以下两个条件的节点：
//     - Auth 值为 0（未设置权限状态）
//     - 没有任何保留的有效子节点（即为叶子节点）
//  4. 保留 Auth 不为 0 或存在有效子节点的节点
//
// 参数：
//   - nodes: 待裁剪的权限树根节点列表
//
// 返回值：
//   - 裁剪后的权限树（去除了所有无意义节点的精简树）
//
// 算法说明：
//
//	采用后序遍历策略：先递归处理所有子节点，根据子节点裁剪结果决定当前节点是否保留。
//	这样可以确保被保留下来的非叶子节点必然存在至少一个有效的子孙节点。
//
// 使用场景：
//   - 提交权限配置前清理：去除用户未操作过的节点
//   - 权限树展示前精简：隐藏未配置权限的中间层节点
//   - 数据库存储前压缩：减少存储的 JSON 数据量
func PruneAuth(nodes []*AuthNode) []*AuthNode {
	// dfs 后序遍历，从叶子向上裁剪
	var dfs func(n *AuthNode) *AuthNode
	dfs = func(n *AuthNode) *AuthNode {
		if n == nil {
			return nil
		}
		// 第一步：递归处理所有子节点，收集有效的子节点
		newChildren := make([]*AuthNode, 0)
		for _, c := range n.Children {
			if nc := dfs(c); nc != nil {
				newChildren = append(newChildren, nc)
			}
		}
		// 第二步：判断是否为叶子节点（所有子节点都被裁剪掉了）
		isLeaf := len(newChildren) == 0
		// 叶子节点且 Auth 为 0 → 无意义节点，返回 nil 表示删除
		if isLeaf && n.Auth == 0 {
			return nil
		}
		// 第三步：保留当前节点，并携带裁剪后的子节点列表
		newNode := &AuthNode{
			Name:     n.Name,
			Urls:     n.Urls,
			Auth:     n.Auth,
			Children: newChildren,
		}

		return newNode
	}
	// 处理根节点数组
	res := make([]*AuthNode, 0)
	for _, n := range nodes {
		if nn := dfs(n); nn != nil {
			res = append(res, nn)
		}
	}
	return res
}

// PruneUnauthorizedAuthTree 根据父权限树裁剪子权限树。
//
// 功能说明：
//
//	根据父角色的权限树（parent）为基准，对子角色的权限树（child）进行裁剪，
//	删除子角色中不在父角色权限范围内的节点。实现"父角色权限回收后，
//	子角色权限自动同步缩小"的核心逻辑。
//
// 处理逻辑：
//  1. 递归遍历子权限树的每个节点
//  2. 在父权限树中查找对应路径的节点
//  3. 如果当前节点在父权限树中不存在（父角色没有该权限），则删除该节点
//  4. 如果是叶子节点且父节点 Auth 不为 1（父角色没有该权限），则删除该节点
//  5. 保留在父权限范围内且满足条件的节点
//
// 参数：
//   - parent: 父角色的权限树（作为裁剪基准，定义了权限的最大边界）
//   - child:  子角色的权限树（待裁剪的权限树）
//
// 返回值：
//   - 裁剪后的子角色有效权限树，确保不超出父角色权限范围
//
// 使用场景：
//   - 父角色权限被回收后，同步裁剪子角色权限
//   - 防止子角色保留越权权限（权限继承时的安全兜底）
//   - 生成子角色最终有效权限树
func PruneUnauthorizedAuthTree(parent, child []*AuthNode) []*AuthNode {
	// find 在权限树中按路径查找节点
	// names 是一个从根到目标节点的名称路径数组
	// 例如：["系统管理", "用户管理", "用户列表"]
	var find func(nodes []*AuthNode, names []string) *AuthNode

	find = func(nodes []*AuthNode, names []string) *AuthNode {
		if len(names) == 0 {
			return nil
		}
		// 在节点列表中查找名称匹配的第一个节点
		for _, n := range nodes {
			if n.Name != names[0] {
				continue
			}
			// 路径已经匹配到最后一层，返回该节点
			if len(names) == 1 {
				return n
			}
			// 继续向子节点查找剩余路径
			return find(n.Children, names[1:])
		}
		return nil
	}

	// dfs 深度优先遍历子权限树进行裁剪
	// path 记录从根到当前节点的名称路径，用于在父树中定位对应节点
	var dfs func(nodes []*AuthNode, path []string) []*AuthNode
	dfs = func(nodes []*AuthNode, path []string) []*AuthNode {
		res := make([]*AuthNode, 0)
		for _, n := range nodes {
			// 构建当前节点的完整路径
			cur := append(append([]string{}, path...), n.Name)
			// 在父权限树中查找对应节点
			p := find(parent, cur)
			// 父权限树中不存在该路径 → 当前节点越权，跳过（删除）
			if p == nil {
				continue
			}
			// 判断是否为叶子节点
			isLeaf := len(n.Children) == 0
			// 叶子节点且父节点 Auth 不为 1（父角色无此权限）→ 跳过（删除）
			if isLeaf && p.Auth != 1 {
				continue
			}
			// 保留当前节点
			newNode := &AuthNode{
				Name: n.Name,
				Urls: n.Urls,
				Auth: n.Auth,
			}
			// 递归裁剪子节点
			newNode.Children = dfs(n.Children, cur)
			res = append(res, newNode)
		}
		return res
	}
	return dfs(child, []string{})
}

// SyncAuthTree 根据父权限树和子权限树，生成子权限编辑选项。
//
// 功能说明：
//
//	遍历父权限树，对每个叶子节点或 Auth 已设置的节点，
//	检查该节点在子权限树中是否存在，从而确定子角色对该权限的状态：
//	  - Auth = 1: 子角色已拥有该权限（在子权限树中存在）
//	  - Auth = 2: 子角色未拥有该权限（在子权限树中不存在）
//	最终生成一份带状态标记的权限编辑树，供前端渲染"可选/已选"界面。
//
// 参数：
//   - parent: 父角色的完整权限树（定义了子角色可以拥有的最大权限范围）
//   - child:  子角色当前的权限树
//
// 返回值：
//   - 直接修改 parent 树并返回（原地修改），每个节点的 Auth 标记了子角色的权限状态
//
// 使用场景：
//   - 权限编辑页面渲染：显示完整权限列表并勾选子角色已有权限
//   - 权限差异对比：快速识别子角色有哪些额外的或缺失的权限
//   - 角色权限配置界面初始化
func SyncAuthTree(parent, child []*AuthNode) []*AuthNode {
	// find 在权限树中按路径查找节点（与 PruneUnauthorizedAuthTree 中的 find 逻辑一致）
	var find func(nodes []*AuthNode, names []string) *AuthNode
	find = func(nodes []*AuthNode, names []string) *AuthNode {
		if len(names) == 0 {
			return nil
		}
		for _, n := range nodes {
			if n.Name != names[0] {
				continue
			}
			if len(names) == 1 {
				return n
			}
			return find(n.Children, names[1:])
		}
		return nil
	}

	// dfs 遍历父权限树，为每个需要标记的节点设置 Auth 状态
	var dfs func(nodes []*AuthNode, path []string)
	dfs = func(nodes []*AuthNode, path []string) {
		for _, n := range nodes {
			// 构建当前节点路径
			cur := append(path, n.Name)
			// 判断是否为"终端节点"：无子节点 或 Auth 已非 0
			// 只有终端节点才需要标记权限状态（中间层节点保留原状用于展开）
			if len(n.Children) == 0 || n.Auth != 0 {
				// 在子权限树中查找对应节点
				if find(child, cur) != nil {
					n.Auth = 1 // 子角色拥有该权限
				} else {
					n.Auth = 2 // 子角色未拥有该权限
				}
			}
			// 递归处理子节点
			dfs(n.Children, cur)
		}
	}
	dfs(parent, []string{})
	return parent
}

// Permissions 将权限树转换为 URL 到权限路径的映射表。
//
// 功能说明：
//
//	递归遍历权限树的所有节点，收集每个节点的 Urls 和从根到该节点的路径，
//	建立 URL → 权限路径列表 的映射关系。
//
// 参数：
//   - nodes: 权限树根节点列表
//
// 返回值：
//   - map[string][]string: key 为 API URL，value 为该 URL 所属的所有权限路径
//
// 返回值示例：
//
//	{
//	  "/api/user/list":   ["系统管理.用户管理.用户列表"],
//	  "/api/user/detail": ["系统管理.用户管理.用户详情"],
//	  "/api/export/data": ["报表管理.数据导出", "系统管理.数据导出"],
//	}
//
// 使用场景：
//   - API 鉴权中间件：根据请求 URL 快速查找需要的权限路径
//   - 权限检查缓存：将树形权限结构转换为 O(1) 查表结构
//   - URL 批量鉴权：一次性建立完整的 URL → 权限 映射关系
func Permissions(nodes []*AuthNode) map[string][]string {
	res := map[string][]string{}
	// dfs 遍历权限树，收集 URL 到路径的映射
	var dfs func(nodes []*AuthNode, path []string)
	dfs = func(nodes []*AuthNode, path []string) {
		for _, n := range nodes {
			// 构建当前节点的完整权限路径（如 "系统管理.用户管理.用户列表"）
			cur := append(path, n.Name)
			key := strings.Join(cur, ".")
			// 将该节点的所有 URL 映射到当前权限路径
			for _, url := range n.Urls {
				res[url] = append(res[url], key)
			}
			// 递归处理子节点
			dfs(n.Children, cur)
		}
	}
	dfs(nodes, []string{})
	return res
}

// Flatten 将权限树扁平化为一层 map 结构。
//
// 功能说明：
//
//	递归遍历权限树，仅提取 Auth 值为 1（有权限）的节点，
//	将其完整路径作为 key、固定值 1 作为 value，构建扁平化映射表。
//
// 参数：
//   - nodes: 权限树根节点列表
//
// 返回值：
//   - map[string]any: key 为具有权限的节点的完整点分隔路径，value 固定为 1
//
// 返回值示例：
//
//	{
//	  "系统管理.用户管理.用户列表": 1,
//	  "系统管理.用户管理.用户详情": 1,
//	  "报表管理.数据导出": 1,
//	}
//
// 使用场景：
//   - Redis 缓存存储：将权限数据序列化为扁平 JSON 存入 Redis Hash
//   - 权限比对：快速判断某个完整路径是否有权限（O(1) 查表）
//   - 数据库中存储用户权限：以 JSON 格式存储用户有权限的路径集合
//   - 前端权限判断：前端只需检查路径是否在扁平 map 中即可
func Flatten(nodes []*AuthNode) map[string]any {
	res := make(map[string]any)
	// dfs 遍历权限树，仅收集 Auth=1 的节点路径
	var dfs func(node *AuthNode, path string)
	dfs = func(node *AuthNode, path string) {
		// 构建当前节点的完整路径
		cur := node.Name
		if path != "" {
			// 非根节点：在前面拼接父路径
			cur = path + "." + node.Name
		}
		// 仅当 Auth 为 1（有权限）时记录
		if node.Auth == 1 {
			res[cur] = 1
		}
		// 递归处理子节点
		for _, child := range node.Children {
			dfs(child, cur)
		}
	}
	// 遍历所有根节点
	for _, n := range nodes {
		dfs(n, "")
	}
	return res
}
