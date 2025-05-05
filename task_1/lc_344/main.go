package main

import "fmt"

// https://leetcode.cn/problems/reverse-string/
func main() {
	byteSlice := []byte{'H', 'e', 'l', 'l', 'o'}
	reverseString(byteSlice)
}

// 简单的头尾交换
func reverseString(s []byte) {
	fmt.Println(string(s))
	for i, _ := range s {
		if i <= len(s)-1-i {
			temp := s[i]
			s[i] = s[len(s)-1-i]
			s[len(s)-1-i] = temp
		}
	}
	fmt.Println(string(s))
}
