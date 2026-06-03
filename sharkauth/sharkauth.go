package sharkauth

import (
	"strings"
)

type AuthNode struct {
	Name     string      `json:"name,omitempty"`
	Children []*AuthNode `json:"children,omitempty"`
	Urls     []string    `json:"urls,omitempty"`
	Auth     int         `json:"auth,omitempty"`
}

// NormalizeAuthTree 深拷贝并规范化权限树：
// 1. 清空所有 Urls 字段
// 2. 对 Urls 不为空的节点设置 Auth 值 0=权限不存在 1=有权限 2=无权限
func NormalizeAuthTree(nodes []*AuthNode, auth int) []*AuthNode {
	var dfs func(n *AuthNode) *AuthNode
	dfs = func(n *AuthNode) *AuthNode {
		if n == nil {
			return nil
		}
		newNode := &AuthNode{
			Name: n.Name,
		}
		if len(n.Urls) > 0 {
			newNode.Auth = auth
		}
		newNode.Urls = nil
		if len(n.Children) > 0 {
			newNode.Children = make([]*AuthNode, 0, len(n.Children))
			for _, child := range n.Children {
				newNode.Children = append(newNode.Children, dfs(child))
			}
		}
		return newNode
	}
	res := make([]*AuthNode, 0, len(nodes))
	for _, n := range nodes {
		res = append(res, dfs(n))
	}
	return res
}

// PruneAuth 递归裁剪权限树：
// 1. 深拷贝权限树，避免修改原数据
// 2. 从叶子节点开始向上裁剪
// 3. 删除 Auth=0 且无子节点的节点
// 4. 保留 Auth!=0 或仍存在有效子节点的节点
func PruneAuth(nodes []*AuthNode) []*AuthNode {
	var dfs func(n *AuthNode) *AuthNode
	dfs = func(n *AuthNode) *AuthNode {
		if n == nil {
			return nil
		}
		newChildren := make([]*AuthNode, 0)
		for _, c := range n.Children {
			if nc := dfs(c); nc != nil {
				newChildren = append(newChildren, nc)
			}
		}
		isLeaf := len(newChildren) == 0
		if isLeaf && n.Auth == 0 {
			return nil
		}
		newNode := &AuthNode{
			Name:     n.Name,
			Urls:     n.Urls,
			Auth:     n.Auth,
			Children: newChildren,
		}

		return newNode
	}
	res := make([]*AuthNode, 0)
	for _, n := range nodes {
		if nn := dfs(n); nn != nil {
			res = append(res, nn)
		}
	}
	return res
}

// PruneUnauthorizedAuthTree 根据 parent 权限树裁剪 child 权限树：
//
// 处理逻辑：
// 1. 递归遍历 child 权限树
// 2. 如果当前节点在 parent 中不存在或者auth!=1，则删除
// 3. 仅保留 parent 中允许存在的权限节点
// 4. 返回裁剪后的 child 最终权限树
//
// 使用场景：
// 1. 父角色权限被回收后，同步裁剪子角色权限
// 2. 防止子角色保留越权权限
// 3. 生成子角色最终有效权限
func PruneUnauthorizedAuthTree(parent, child []*AuthNode) []*AuthNode {
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
	var dfs func(nodes []*AuthNode, path []string) []*AuthNode
	dfs = func(nodes []*AuthNode, path []string) []*AuthNode {
		res := make([]*AuthNode, 0)
		for _, n := range nodes {
			cur := append(append([]string{}, path...), n.Name)
			p := find(parent, cur)
			if p == nil {
				continue
			}
			isLeaf := len(n.Children) == 0
			if isLeaf && p.Auth != 1 {
				continue
			}
			newNode := &AuthNode{
				Name: n.Name,
				Urls: n.Urls,
				Auth: n.Auth,
			}
			newNode.Children = dfs(n.Children, cur)
			res = append(res, newNode)
		}
		return res
	}
	return dfs(child, []string{})
}

// SyncAuthTree 根据父权限树和子权限树,生成子权限编辑选项
func SyncAuthTree(parent, child []*AuthNode) []*AuthNode {
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
	var dfs func(nodes []*AuthNode, path []string)
	dfs = func(nodes []*AuthNode, path []string) {
		for _, n := range nodes {
			cur := append(path, n.Name)
			if len(n.Children) == 0 || n.Auth != 0 {
				if find(child, cur) != nil {
					n.Auth = 1
				} else {
					n.Auth = 2
				}
			}
			dfs(n.Children, cur)
		}
	}
	dfs(parent, []string{})
	return parent
}

// Permissions 将权限树转换为 URL 到权限路径的映射
func Permissions(nodes []*AuthNode) map[string][]string {
	res := map[string][]string{}
	var dfs func(nodes []*AuthNode, path []string)
	dfs = func(nodes []*AuthNode, path []string) {
		for _, n := range nodes {
			cur := append(path, n.Name)
			key := strings.Join(cur, ".")
			for _, url := range n.Urls {
				res[url] = append(res[url], key)
			}
			dfs(n.Children, cur)
		}
	}
	dfs(nodes, []string{})
	return res
}

// Flatten 将权限树扁平化
func Flatten(nodes []*AuthNode) map[string]any {
	res := make(map[string]any)
	var dfs func(node *AuthNode, path string)
	dfs = func(node *AuthNode, path string) {
		cur := node.Name
		if path != "" {
			cur = path + "." + node.Name
		}
		if node.Auth == 1 {
			res[cur] = 1
		}
		for _, child := range node.Children {
			dfs(child, cur)
		}
	}
	for _, n := range nodes {
		dfs(n, "")
	}
	return res
}
