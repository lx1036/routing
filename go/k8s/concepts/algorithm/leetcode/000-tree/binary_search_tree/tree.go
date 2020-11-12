package binary_search_tree

import "fmt"

// Binary Search Tree

type Node struct {
	value int
	left *Node
	right *Node
}

func (node *Node) find(value int) *Node {
	current := node
	for current != nil {
		if value > current.value {
			current = current.right
		} else if value < current.value {
			current = current.left
		} else if value == current.value {
			return current
		}
	}
	
	return nil
}

// 递归实现插入节点，很巧妙
// 数据大于当前节点值，插入右节点；数据小于当前节点值，插入左节点
func (node *Node) insert(value int) error {
	if value == node.value {
		return nil
	}
	
	if value > node.value {
		if node.right == nil {
			node.right = &Node{value: value}
		} else {
			node.right.insert(value)
		}
	} else if value < node.value {
		if node.left == nil {
			node.left = &Node{value: value}
		} else {
			node.left.insert(value)
		}
	}
	
	return nil
}

func (node *Node) delete(value int)  {

}

// 遍历树：根据特定顺序遍历树的每一个节点
// 中序遍历：左子树 -> 根节点 -> 右子树
// 前序遍历：根节点 -> 左子树 -> 右子树
// 后序遍历：左子树 -> 右子树 -> 根节点

func (node *Node) MiddleOrder()  {
	current := node
	if current != nil {
		current.left.MiddleOrder()
		fmt.Println(current.value)
		current.right.MiddleOrder()
	}
}

func (node *Node) PreOrder()  {
	current := node
	if current != nil {
		fmt.Println(current.value)
		current.left.MiddleOrder()
		current.right.MiddleOrder()
	}
}

func (node *Node) PostOrder()  {
	current := node
	if current != nil {
		current.left.MiddleOrder()
		current.right.MiddleOrder()
		fmt.Println(current.value)
	}
}

func (node *Node) Min() {
	
}

func (node *Node) Max() {

}
