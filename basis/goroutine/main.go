package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {
	//account()
	dispatcher()
}

// 题目 ：编写一个程序，使用 go 关键字启动两个协程，一个协程打印从1到10的奇数，另一个协程打印从2到10的偶数。
// 考察点 ： go 关键字的使用、协程的并发执行。
// * - defer 确保无论函数如何退出（正常返回或 panic）都会执行
// * - Done() 通知 WaitGroup 该 goroutine 已完成
// 最佳实践建议:
// 1.总是使用 defer wg.Done()：确保在任何情况下都会调用
// 2.Add 和 Done 要匹配：每个 Add(1) 必须对应一个 Done()
// 3.考虑将 WaitGroup 作为参数传递：对于更复杂的场景
// 4.避免在 goroutine 内部调用 Add：容易导致竞态条件
func account() {
	var wg sync.WaitGroup
	wg.Add(2) // 等待两个协程完成

	go func() {
		defer wg.Done()
		for i := 1; i <= 10; i++ {
			if i%2 == 0 {
				fmt.Println("偶数", i)
			}
		}
	}()

	go func() {
		defer wg.Done()
		for i := 1; i <= 10; i++ {
			if i%2 != 0 {
				fmt.Println("奇数", i)
			}
		}
	}()

	wg.Wait() // 等待所有协程完成
}

// 题目 ：设计一个任务调度器，接收一组任务（可以用函数表示），并使用协程并发执行这些任务，同时统计每个任务的执行时间。
// 考察点 ：协程原理、并发任务调度。
func dispatcher() {
	var wg sync.WaitGroup
	wg.Add(4) // 等待两个协程完成

	go func() {
		defer wg.Done()
		start := time.Now()
		study()
		end := time.Now()
		fmt.Println("study任务执行时间为", end.Sub(start))
	}()

	go func() {
		defer wg.Done()
		start := time.Now()
		sleep()
		end := time.Now()
		fmt.Println("sleep任务执行时间为", end.Sub(start))
	}()

	go func() {
		defer wg.Done()
		start := time.Now()
		eat()
		end := time.Now()
		fmt.Println("eat任务执行时间为", end.Sub(start))
	}()

	go func() {
		defer wg.Done()
		start := time.Now()
		smoke()
		end := time.Now()
		fmt.Println("smoke任务执行时间为", end.Sub(start))
	}()

	wg.Wait() // 等待所有协程完成
}

func study() {
	fmt.Println("学习...")
	time.Sleep(2 * time.Second)
}

func sleep() {
	fmt.Println("睡觉...")
	time.Sleep(2 * time.Second)
}

func eat() {
	fmt.Println("吃饭...")
	time.Sleep(1 * time.Second)
}

func smoke() {
	fmt.Println("吸烟...")
	time.Sleep(3 * time.Second)
}
