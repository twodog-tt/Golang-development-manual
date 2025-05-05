package main

import (
	"fmt"
	"sync"
)

func main() {
	secondChannel()
}

// 题目 ：编写一个程序，使用通道实现两个协程之间的通信。一个协程生成从1到10的整数，并将这些整数发送到通道中，
// 另一个协程从通道中接收这些整数并打印出来。
// 考察点 ：通道的基本使用、协程间通信。

func firstChannel() {
	ch1 := make(chan int, 10)
	wg := &sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer close(ch1)
		for i := 1; i <= 10; i++ {
			fmt.Println("in ch1:", i)
			ch1 <- i
		}
	}()

	go func() {
		defer wg.Done()
		for num := range ch1 { // 使用range自动检测通道关闭
			fmt.Println("out ch1:", num)
		}
	}()

	wg.Wait()
}

// 实现一个带有缓冲的通道，生产者协程向通道中发送100个整数，消费者协程从通道中接收这些整数并打印。
// 考察点 ：通道的缓冲机制。
func secondChannel() {
	ch1 := make(chan int, 100)
	wg := &sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer close(ch1)
		for i := 1; i <= 100; i++ {
			fmt.Println("in ch1:", i)
			ch1 <- i
		}
	}()

	go func() {
		defer wg.Done()
		for num := range ch1 {
			fmt.Println("out ch1:", num)
		}
	}()

	wg.Wait()
}
