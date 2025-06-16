package questions

import (
	"fmt"
	"hash/fnv"
	"strconv"
	"sync"
)

/*
Go的Map是否并发安全？如何实现并发安全？
*/

func testMap() {
	var m map[int]string
	m[1] = "hello" // 对没有初始化的map进行赋值操作会抛错
	fmt.Println(m[1])

	/*
		panic: assignment to entry in nil map [recovered]
		panic: assignment to entry in nil map
	*/
}
func testMapFix() {
	m := make(map[int]string)
	m[1] = "hello"
	fmt.Println(m[1])
}

// fatal error: concurrent map writes
func testMap1() {
	m := make(map[int]int)
	wg := sync.WaitGroup{}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m[i] = i // ❌ 并发写 map，会引发 fatal error
		}(i)
	}

	wg.Wait()
	fmt.Println("map 长度：", len(m))
}

// 使用 sync.Mutex 加锁:对map操作前加锁，对map操作后解锁
func testMap1Fix() {
	m := make(map[int]int)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mu.Lock()
			m[i] = i
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	fmt.Println("map 长度：", len(m)) // map 长度： 100
}

// 使用 sync.Map（Go 1.9+） 性能好，适合读多写少或缓存场景
func testMap1Fix1() {
	var m sync.Map
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m.Store(i, i)
		}(i)
	}

	wg.Wait()

	count := 0
	m.Range(func(key, value any) bool {
		count++
		return true
	})

	fmt.Println("map 长度：", count) // map 长度： 100
}

const shardCount = 32 // 分片数量

/*用分片 Map（Sharded Map）来对map进行并发写入*/
// 每个分片结构
type shard struct {
	sync.RWMutex
	data map[string]string
}

// ShardedMap 是并发安全的 map
type ShardedMap struct {
	shards [shardCount]*shard
}

// NewShardedMap 构造函数
func NewShardedMap() *ShardedMap {
	m := &ShardedMap{}
	for i := 0; i < shardCount; i++ {
		m.shards[i] = &shard{
			data: make(map[string]string),
		}
	}
	return m
}

// hash 函数：把 key 映射到某个分片索引
func getShardIndex(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() % shardCount)
}

// 获取某个分片
func (m *ShardedMap) getShard(key string) *shard {
	return m.shards[getShardIndex(key)]
}

// Set 设置 key-value
func (m *ShardedMap) Set(key, value string) {
	s := m.getShard(key)
	s.Lock()
	defer s.Unlock()
	s.data[key] = value
}

// Get 获取 key 的值
func (m *ShardedMap) Get(key string) (string, bool) {
	s := m.getShard(key)
	s.RLock()
	defer s.RUnlock()
	val, ok := s.data[key]
	return val, ok
}

// Delete 删除 key
func (m *ShardedMap) Delete(key string) {
	s := m.getShard(key)
	s.Lock()
	defer s.Unlock()
	delete(s.data, key)
}

func testMap2() {
	sm := NewShardedMap()
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "key" + strconv.Itoa(i)
			value := "val" + strconv.Itoa(i)
			sm.Set(key, value)
		}(i)
	}

	wg.Wait()

	v, ok := sm.Get("key123")
	fmt.Println("key123:", v, ok)
}
