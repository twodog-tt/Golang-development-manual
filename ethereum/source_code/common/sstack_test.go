package common

import (
	"testing"

	"github.com/ethereum/go-ethereum/common/prque"
)

// 用于测试的数据类型
type testItem struct {
	id   int
	prio int64
}

// SetIndex 回调函数的模拟实现
func mockSetIndex[V any](v V, index int) {
	// 这里可以添加额外逻辑来记录或验证索引更新
}

// TestSstackCoreFunctions 测试 sstack 的基本操作
func TestSstackCoreFunctions(t *testing.T) {
	// 创建一个新的优先队列
	q := prque.New[int64, testItem](mockSetIndex[testItem])

	// Push 几个元素 原则：后进先出
	q.Push(testItem{id: 1, prio: 10}, 10)
	q.Push(testItem{id: 2, prio: 5}, 5)
	q.Push(testItem{id: 3, prio: 15}, 15)

	t.Log(q)
	// 检查队列大小
	if q.Size() != 3 {
		t.Errorf("预期队列大小为 3，实际为 %d", q.Size())
	}

	// Peek 最高优先级元素 (prio=15)
	item, prio := q.Peek() // 查看优先队列中优先级最高的元素，但不将其弹出队列。
	if prio != 15 || item.id != 3 {
		t.Errorf("Peek 返回了错误的元素: id=%d, priority=%d", item.id, prio)
	}

	// Pop 第一个元素
	poppedItem, poppedPrio := q.Pop() // 从优先队列中移除并返回优先级最高的元素。
	if poppedPrio != 15 || poppedItem.id != 3 {
		t.Errorf("Pop 返回了错误的元素: id=%d, priority=%d", poppedItem.id, poppedPrio)
	}

	// 再次检查队列大小
	if q.Size() != 2 {
		t.Errorf("预期队列大小为 2，实际为 %d", q.Size())
	}

	// Pop 第二个元素
	poppedItem, poppedPrio = q.Pop()
	if poppedPrio != 10 || poppedItem.id != 1 {
		t.Errorf("Pop 返回了错误的元素: id=%d, priority=%d", poppedItem.id, poppedPrio)
	}

	// Pop 第三个元素
	poppedItem, poppedPrio = q.Pop()
	if poppedPrio != 5 || poppedItem.id != 2 {
		t.Errorf("Pop 返回了错误的元素: id=%d, priority=%d", poppedItem.id, poppedPrio)
	}

	// 检查队列是否为空
	if !q.Empty() {
		t.Errorf("预期队列为 empty")
	}
}
