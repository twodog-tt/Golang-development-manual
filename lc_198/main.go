package main

import "fmt"

// https://leetcode.cn/problems/house-robber/description/
func main() {
	nums := []int{4, 1, 2, 1, 2}
	fmt.Println(rob(nums))
}

// 递归搜索+保存计算结果=记忆化搜索
func rob(nums []int) int {
	if len(nums) == 1 {
		return nums[0]
	}
	dp := make([]int, len(nums))
	dp[0] = nums[0]
	dp[1] = getMax(nums[0], nums[1])
	// 为dp这个保存计算结果的容器赋予初始值，在给定数组长度大于2的情况下
	for i := 2; i < len(nums); i++ {
		// 从给定数组第三个元素下标开始遍历
		// 根据dp[1]推导出求最大值公式
		// 此时dp[i]中存储的是，数组在前i个元素（包含第i个）的符合条件的最大合
		dp[i] = getMax(dp[i-2]+nums[i], dp[i-1])
	}
	return dp[len(nums)-1]
}

func getMax(x, y int) int {
	if x > y {
		return x
	}
	return y
}
