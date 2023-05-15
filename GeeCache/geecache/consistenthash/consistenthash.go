package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

// Hash函数，通过key（参数是data）计算值的存储位值
type Hash func(data []byte) uint32

type Map struct {
	//对应的哈希函数
	hash Hash
	//虚拟节点倍数
	replicas int
	//哈希环
	keys []int
	//虚拟节点与真实节点之间的映射表
	//键是虚拟节点的哈希值，值是真实节点的名称
	hashMap map[int]string
}

// 自定义虚拟节点倍数和 Hash 函数。
func New(replicas int, f Hash) *Map {
	m := &Map{
		hash:     f,
		replicas: replicas,
		hashMap:  make(map[int]string),
	}
	if m.hash == nil {
		//默认为 crc32.ChecksumIEEE 算法
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

// 添加“真实”节点/机器的Add
// 允许传入 0 或 多个真实节点的名称
func (m *Map) Add(keys ...string) {
	for _, key := range keys {
		//对每一个真实的节点key，创建replicas个虚拟节点
		for i := 0; i < m.replicas; i++ {
			//虚拟节点的名称：编号+key
			//m.hash() 计算虚拟节点的哈希值
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			//添加到环上
			m.keys = append(m.keys, hash)
			//增加虚拟与真实之间的映射关系
			//键是虚拟的，值是真实的
			m.hashMap[hash] = key
		}
	}
	//环上的哈希值排序
	sort.Ints(m.keys)
}

// 选择节点的 Get() 方法
func (m *Map) Get(key string) string {
	if len(m.keys) == 0 {
		return ""
	}
	//m.hash计算key的哈希值
	hash := int(m.hash([]byte(key)))
	//顺时针查找到第一个匹配的虚拟节点的下标
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})
	//通过hashMap映射到真实的节点
	//当idx==len(m.keys)的时候，应该选择m.keys[0],必须用取余的方式处理
	return m.hashMap[m.keys[idx%len(m.keys)]]
}
