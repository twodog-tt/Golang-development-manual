package main

import "fmt"

func main() {
	//x := 10
	//fmt.Println("原始值：", x)
	//addNum(&x)
	//fmt.Println("修改后：", x)

	nums := []int{1, 2, 3}
	fmt.Println("修改前:", nums) // [1 2 3]
	mulNum(&nums)
	fmt.Println("修改后:", nums) // [2 4 6]
}

// 编写一个Go程序，定义一个函数，该函数接收一个整数指针作为参数，在函数内部将该指针指向的值增加10，然后在主函数中调用该函数并输出修改后的值。
// 考察点 ：指针的使用、值传递与引用传递的区别。
// & - 取地址
// * - 解引用，访问指针指向的值（未初始化的指针是nil，解引会导致Panic）
func addNum(x *int) {
	*x = *x + 10
}

// 题目 ：实现一个函数，接收一个整数切片的指针，将切片中的每个元素乘以2。
// 考察点 ：指针运算、切片操作。
// 切片是引用类型，但它的元素访问是值拷贝
// 要修改元素必须通过索引直接访问
// 指针解引用后操作更清晰
// range循环中的变量是值的拷贝
func mulNum(x *[]int) {
	s := *x
	for i := range s {
		s[i] *= 2
	}
}
