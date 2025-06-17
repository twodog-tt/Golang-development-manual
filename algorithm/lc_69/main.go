package main

import "fmt"

// https://leetcode.cn/problems/sqrtx/
func main() {
	fmt.Println(mySqrt(888888))
}

func mySqrt(x int) int {
	// 利用二分法找到最接近x的整数
	// 已知：0 <= x <= 2的31次幂 - 1,那么x的平方根的范围就是有限的
	// 对于有限数列中寻找满足条件的结果，可以用穷举法
	// 那么 对穷举法的优化算法 就是二分法
	// res的范围是 0<=res<=2的16次方-1（65535）
	// 使用二分法对0-65535这个集合中的元素进行平方计算 最接近x的那个数就是结果
	if x == 0 {
		return 0
	}
	low, high := 1, x
	for low < high {
		mid := low + (high-low+1)/2
		if int64(mid)*int64(mid) <= int64(x) {
			low = mid
		} else {
			high = mid - 1
		}
	}
	return low
}
