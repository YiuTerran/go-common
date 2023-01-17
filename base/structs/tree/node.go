package tree

import (
	"github.com/YiuTerran/go-common/base/structs/queue"
)

// Node 是任意多叉树的结点
// 考虑到后续标准库里面会加入链表、队列等基本数据结构，这里就先不展开写了
type Node[T comparable] struct {
	Data     T          `json:"data"`
	Children []*Node[T] `json:"children"`
	Parent   *Node[T]   `json:"-"`
}

type Tree[T comparable] struct {
	//根节点
	Root *Node[T] `json:"root"`
}

func (tree *Tree[T]) CountNodes() int {
	cnt := 0
	if tree.Root != nil {
		cnt = countNode(tree.Root)
	}
	return cnt
}

func countNode[T comparable](node *Node[T]) int {
	numOfNodes := len(node.Children)
	for _, child := range node.Children {
		numOfNodes += countNode(child)
	}
	return numOfNodes
}

// PreOrderTraversal 前序遍历树
func PreOrderTraversal[T comparable](tree *Tree[T], f func(*Node[T], int)) {
	var fn func(node *Node[T], depth int)
	fn = func(node *Node[T], depth int) {
		if node == nil {
			return
		}
		f(node, depth)
		if len(node.Children) > 0 {
			for _, child := range node.Children {
				fn(child, depth+1)
			}
		}
	}
	fn(tree.Root, 0)
}

// PostOrderTraversal 后序遍历树
func PostOrderTraversal[T comparable](tree *Tree[T], f func(*Node[T], int)) {
	var fn func(node *Node[T], depth int)
	fn = func(node *Node[T], depth int) {
		if node == nil {
			return
		}
		if len(node.Children) > 0 {
			for _, child := range node.Children {
				fn(child, depth+1)
			}
		}
		f(node, depth)
	}
	fn(tree.Root, 0)
}

type levelNode[T comparable] struct {
	node  *Node[T]
	level int
}

//LevelDownTraversal 从上到下层级遍历
func LevelDownTraversal[T comparable](tree *Tree[T], f func(*Node[T], int)) {
	if tree == nil || tree.Root == nil {
		return
	}
	q := queue.New[*levelNode[T]]()
	q.Add(&levelNode[T]{
		node:  tree.Root,
		level: 0,
	})
	for {
		if q.Length() == 0 {
			break
		}
		ln := q.Pop()
		//将队列
		for _, child := range ln.node.Children {
			q.Add(&levelNode[T]{
				node:  child,
				level: ln.level + 1,
			})
		}
		f(ln.node, ln.level)
	}
}
