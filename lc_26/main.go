package main

import "fmt"

// https://leetcode.cn/problems/remove-duplicates-from-sorted-array/
func main() {
	arr := []int{0, 0, 1, 1, 1, 2, 2, 3, 3, 4} // 输入数组
	fmt.Println(removeDuplicates(arr))
}

func removeDuplicates(nums []int) int {
	if len(nums) == 0 {
		return 0
	}

	k := 0                           // 慢指针，指向当前唯一元素的最后一个位置
	for i := 1; i < len(nums); i++ { // 快指针遍历数组
		if nums[i] != nums[k] {
			k++
			nums[k] = nums[i] // 将新发现的唯一元素移到前面
		}
	}
	return k + 1 // 返回唯一元素的个数
}
