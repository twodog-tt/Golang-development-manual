package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"
)

func main() {
	counter()
	counter2()
}

// 题目 ：编写一个程序，使用 sync.Mutex 来保护一个共享的计数器。启动10个协程，每个协程对计数器进行1000次递增操作，最后输出计数器的值。
// 考察点 ： sync.Mutex 的使用、并发数据安全。
var (
	count  int
	count2 int
	wg     sync.WaitGroup
	mu     sync.Mutex // 互斥锁保护计数器
)

func counter() {
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				mu.Lock()   // 加锁
				count++     // 解锁
				mu.Unlock() // 安全递增
			}
		}()
	}

	wg.Wait()
	fmt.Println("count:", count)
}

// 使用原子操作（ sync/atomic 包）实现一个无锁的计数器。启动10个协程，每个协程对计数器进行1000次递增操作，最后输出计数器的值。
// 考察点 ：原子操作、并发数据安全。

func counter2() {
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				atomic.AddInt64((*int64)(unsafe.Pointer(&count2)), 1)
			}
		}()
	}
	wg.Wait()
	fmt.Println("count2 no lock:", count2)
}
