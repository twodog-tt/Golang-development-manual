package main

// https://leetcode.cn/problems/merge-two-sorted-lists/
func main() {
	var list1, list2 *ListNode
	mergeTwoLists(list1, list2)
}

/**
 * Definition for singly-linked list.
 */
func mergeTwoLists(list1 *ListNode, list2 *ListNode) *ListNode {
	var head, pre, cur1, cur2 *ListNode
	if list1.Val <= list2.Val {
		head = list1
		cur2 = list2
	} else {
		head = list2
		cur2 = list1
	}
	cur1 = head.Next
	pre = head

	for cur1 != nil && cur2 != nil {
		if cur1.Val <= cur2.Val {
			pre.Next = cur1
			cur1 = cur1.Next
		} else {
			pre.Next = cur2
			cur2 = cur2.Next
		}
		pre = pre.Next
	}
	if cur1 != nil {
		pre.Next = cur1
	} else {
		pre.Next = cur2
	}
	return head
}

type ListNode struct {
	Val  int
	Next *ListNode
}
