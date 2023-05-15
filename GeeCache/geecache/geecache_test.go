package geecache

import (
	"fmt"
	"log"
	"reflect"
	"testing"
)

func TestGetter(t *testing.T) {
	//首先参数是一个匿名函数
	//借助GetterFunc的类型转化，将此匿名函数转化成GetterFunc类型
	//最后将此类型转化成接口
	var now Getter = GetterFunc(func(key string) ([]byte, error) {
		return []byte(key), nil
	})

	expect := []byte("key")
	v, _ := now.Get("key")
	if !reflect.DeepEqual(v, expect) {
		t.Errorf("callback failed")
	}

	//var n GetterFunc = GetterFunc(func(key string) ([]byte, error) {
	//	return []byte(key), nil
	//})
}

// 用一个 map 模拟耗时的数据库。
var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

func TestGet(t *testing.T) {
	loadCounts := make(map[string]int, len(db))
	gee := NewGroup("scores", 2<<10, GetterFunc(
		func(key string) ([]byte, error) {
			log.Println("[SlowDB] search key", key)
			//从数据源中查找
			if v, ok := db[key]; ok {
				//在数据源中找到，统计某个键调用回调函数的次数
				if _, ok := loadCounts[key]; !ok {
					//log.Fatal(loadCounts[key])
					loadCounts[key] = 0
				} //次数++
				loadCounts[key] += 1
				//fmt.Println(v)
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))

	for k, v := range db {
		//在使用Get的时候调用回调函数
		if view, err := gee.Get(k); err != nil || view.String() != v {
			t.Fatal("failed to get value of Tom")
		} // load from callback function

		if _, err := gee.Get(k); err != nil || loadCounts[k] > 1 {
			//t.Fatal(err, loadCounts[k])
			t.Fatalf("cache %s miss", k)
		} // cache hit
	}

	view, err := gee.Get("unknown")
	if err == nil {
		t.Fatalf("the value of unknow should be empty, but %s got", view)
	}

}
