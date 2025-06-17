package main

import "fmt"

// https://leetcode.cn/problems/permutations/
func main() {
	arr := []int{1, 2, 3, 4}
	// fmt.Println("长度：", len(permute(arr)))
	res := permute(arr)
	fmt.Println("集合：", res)
	fmt.Println("长度：", len(res))
}

func permute(nums []int) [][]int {
	var res [][]int
	backtrack(&res, nums, 0)
	return res
}

func backtrack(res *[][]int, nums []int, start int) {
	if start == len(nums) {
		// 复制当前排列到结果中
		tmp := make([]int, len(nums))
		copy(tmp, nums)
		*res = append(*res, tmp)
		return
	}

	for i := start; i < len(nums); i++ {
		// 交换元素
		nums[start], nums[i] = nums[i], nums[start]
		// 递归处理下一个位置
		backtrack(res, nums, start+1)
		// 回溯，恢复交换
		nums[start], nums[i] = nums[i], nums[start]
	}
}
