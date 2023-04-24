package lru

import "container/list"

type Cache struct {
	//允许使用的最大内存
	maxBytes int64
	//当前已经使用的最大内存
	useBytes int64
	//go的标准库实现双向链表
	ll *list.List
	//字典值，key时string，值是双向链表中对应结点的指针
	cache map[string]*list.Element
	//某条记录被移除时的回调函数
	OnEvicted func(key string, value Value)
}

// 键值对 entry 是双向链表节点的数据类型
type entry struct {
	key   string
	value Value
}

// 为了通用性，我们允许值是实现了 Value 接口的任意类型
type Value interface {
	Len() int
}

//工厂模式实例化
func New(max int64, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:  max,
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		OnEvicted: onEvicted,
	}
}

//获取添加了多少数据
func (c *Cache) Len() int {
	return c.ll.Len()
}

//查找元素，从字典中查到对应链表中的结点
func (c *Cache) Get(key string) (value Value, ok bool) {
	ele, ok := c.cache[key]
	if ok {
		//将结点ele移动到队尾，约定front是队尾
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		return kv.value, true
	}
	return
}

//移除元素，就是缓存淘汰，移除最近最少访问的结点（队首）
func (c *Cache) Remove() {
	//首先拿到队首结点，是个临时变量
	ele := c.ll.Back()
	if ele != nil {
		//将双向链表中的结点删除
		c.ll.Remove(ele)
		kv := ele.Value.(*entry)
		//从字典中删除该结点的映射关系
		delete(c.cache, kv.key)
		//更新当前已经使用的内存
		c.useBytes -= int64(len(kv.key)) + int64(kv.value.Len())
		//调用回调函数
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

//新增/修改
func (c *Cache) Add(key string, value Value) {
	//如果此key-value存在，则更新值
	if ele, ok := c.cache[key]; ok {
		//链表中对应的结点移动到队尾
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		c.useBytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
	} else {
		//不存在此key-value，则新增
		ele := c.ll.PushFront(&entry{key, value})
		c.cache[key] = ele
		c.useBytes += int64(len(key)) + int64(value.Len())
	}
	//如果更新超过了最大值，则移除最少访问的结点
	for c.maxBytes != 0 && c.maxBytes < c.useBytes {
		c.Remove()
	}
}
