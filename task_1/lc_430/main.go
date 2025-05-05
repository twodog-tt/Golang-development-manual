package main

// https://leetcode.cn/problems/flatten-a-multilevel-doubly-linked-list/
func main() {

}

type Node struct {
	Val   int
	Prev  *Node
	Next  *Node
	Child *Node
}

func dfs(node *Node) (last *Node) {
	cur := node
	for cur != nil {
		next := cur.Next
		// 如果有子节点，那么首先处理子节点
		if cur.Child != nil {
			childLast := dfs(cur.Child)
			next = cur.Next
			// 将 node 与 child 相连
			cur.Next = cur.Child
			cur.Child.Prev = cur
			// 如果 next 不为空，就将 last 与 next 相连
			if next != nil {
				childLast.Next = next
				next.Prev = childLast
			}
			// 将 child 置为空
			cur.Child = nil
			last = childLast
		} else {
			last = cur
		}
		cur = next
	}
	return
}

func flatten(root *Node) *Node {
	dfs(root)
	return root
}
