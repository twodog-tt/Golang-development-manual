package common

import (
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/common/prque"
)

// 定义测试结构体
type test1Item struct {
	id       int
	priority int // 实际优先级
	maxUntil mclock.AbsTime
}

// 定义测试结构体方法
func (t test1Item) String() string {
	return fmt.Sprintf("Item{id:%d, priority:%d}", t.id, t.priority)
}

// 模拟回调函数
func setIndexCallback(item test1Item, index int) {
	// 可以用来记录或更新索引，这里仅作占位
}

// 实际优先级回调
func priorityCallback(item test1Item) int {
	return item.priority
}

// 最大预估优先级回调
func maxPriorityCallback(item test1Item, until mclock.AbsTime) int {
	if item.maxUntil >= until {
		return item.priority + 100 // 假设未来一段时间内优先级会提升
	}
	return item.priority
}

// TestLazyQueue_BasicOperations 测试 LazyQueue 的基本操作
func TestLazyQueue_BasicOperations(t *testing.T) {
	mockClock := &mclock.Simulated{}
	q := prque.NewLazyQueue[int, test1Item](
		setIndexCallback,
		priorityCallback,
		maxPriorityCallback,
		mockClock,
		time.Second*5, // refreshPeriod
	)

	// Push 几个元素
	now := mockClock.Now()
	item1 := test1Item{id: 1, priority: 10, maxUntil: now.Add(time.Second * 10)}
	item2 := test1Item{id: 2, priority: 5, maxUntil: now.Add(time.Second * 1)}
	item3 := test1Item{id: 3, priority: 15, maxUntil: now.Add(time.Second * 5)}

	q.Push(item1)
	q.Push(item2)
	q.Push(item3)

	if q.Size() != 3 {
		t.Errorf("预期队列大小为 3，实际为 %d", q.Size())
	}

	// Pop 第一个元素
	popped := q.PopItem()
	if popped.id != 3 {
		t.Errorf("Pop 返回了错误的元素: id=%d", popped.id)
	}

	// 更新 item1 的状态
	item1.priority = 20 // 提升优先级
	q.Update(0)         // 更新原 index=0 的 item

	// 再次 Pop
	popped = q.PopItem()
	if popped.id != 1 {
		t.Errorf("Pop 返回了错误的元素: id=%d", popped.id)
	}

	// Pop 剩余项
	popped = q.PopItem()
	if popped.id != 2 {
		t.Errorf("Pop 返回了错误的元素: id=%d", popped.id)
	}

	// 队列应为空
	if !q.Empty() {
		t.Errorf("预期队列为 empty")
	}
}
