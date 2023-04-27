package singleflight

import "sync"

// 代表正在进行中或者已经结束的请求
type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

// 管理不同key的请求call
type Group struct {
	mutex sync.Mutex
	m     map[string]*call
}

// 第一个参数是key，第二个参数是函数调用。
// Do 的作用就是，针对相同的 key，
// 无论 Do 被调用多少次，函数 f 都只会被调用一次，等待 f 调用结束了，返回返回值或错误
func (g *Group) Do(key string, f func() (interface{}, error)) (interface{}, error) {
	g.mutex.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok {
		//如果请求正在进行中，则等待
		//保证所有的请求都只会被调用一次，可以重复返回结果
		c.wg.Wait()
		//请求结束，返回结果
		return c.val, c.err
	}
	c := new(call)
	//发起请求之前加锁
	c.wg.Add(1)
	//添加到 g.m，表明 key 已经有对应的请求在处理
	g.m[key] = c
	g.mutex.Unlock()
	//调用 f，发起请求
	c.val, c.err = f()
	//请求结束
	c.wg.Done()
	g.mutex.Lock()
	// 更新 g.m
	delete(g.m, key)
	// 返回结果
	g.mutex.Unlock()

	return c.val, c.err

}
