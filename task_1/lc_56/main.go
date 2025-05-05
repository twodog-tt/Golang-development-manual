package main

import (
	"fmt"
	"slices"
	"sort"
)

// https://leetcode.cn/problems/merge-intervals/
func main() {
	var arr [][]int
	arr = append(arr, []int{1, 3})
	arr = append(arr, []int{2, 6})
	arr = append(arr, []int{8, 10})
	arr = append(arr, []int{15, 18})
	fmt.Println(arr)
	res := merge(arr)
	fmt.Println(res)
}
func merge(intervals [][]int) [][]int {
	// 首先需要将数组按照左边临界值进行排序
	// 左边临界值相等的按照右边临界值排序
	// 快慢指针遍历排序后的二维数组，相邻数组对比左边临界值与右边临界值的大小
	// 推导公式，[l1,r1],[l2,r2], r1>=l2 && r2>=r1 ,此时两集合重叠合并为[l1,r2]
	if len(intervals) == 0 {
		return intervals
	}

	// 先按区间起始点排序
	sort.Slice(intervals, func(i, j int) bool {
		return intervals[i][0] < intervals[j][0]
	})

	var result [][]int
	result = append(result, intervals[0])

	for i := 1; i < len(intervals); i++ {
		last := result[len(result)-1]
		current := intervals[i]

		// 如果当前区间与最后一个结果区间重叠
		if current[0] <= last[1] {
			// 合并区间
			if current[1] > last[1] {
				last[1] = current[1]
			}
			result[len(result)-1] = last
		} else {
			result = append(result, current)
		}
	}

	return result
}
func merge1(intervals [][]int) (ans [][]int) {
	slices.SortFunc(intervals, func(p, q []int) int { return p[0] - q[0] }) // 按照左端点从小到大排序
	for _, p := range intervals {
		m := len(ans)
		if m > 0 && p[0] <= ans[m-1][1] { // 可以合并
			ans[m-1][1] = max(ans[m-1][1], p[1]) // 更新右端点最大值
		} else { // 不相交，无法合并
			ans = append(ans, p) // 新的合并区间
		}
	}
	return
}
