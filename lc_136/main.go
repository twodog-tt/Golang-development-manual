package main

import "fmt"

// https://leetcode.cn/problems/single-number/
func main() {
	nums := []int{4, 1, 2, 1, 2}
	fmt.Println(singleNumber(nums))
}

func singleNumber(nums []int) int {
	// 数组从头到尾做减法 取反
	var a = nums[0]
	var i = 1
	for ; i < len(nums); i++ {
		a ^= nums[i]
	}
	return a
}
