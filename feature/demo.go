package main

import (
	"log"
	"reflect"
	"strings"
	"sync"
)

// 使用反射动态得到方法的参数和返回值
func main() {
	var wg sync.WaitGroup
	//首先得到一个普遍interface类型
	typ := reflect.TypeOf(&wg)
	//返回该interface中方法的个数
	for i := 0; i < typ.NumMethod(); i++ {
		//返回第i个方法
		method := typ.Method(i)
		//返回函数参数个数
		argv := make([]string, 0, method.Type.NumIn())
		//返回函数返回值个数
		returns := make([]string, 0, method.Type.NumOut())
		// j 从 1 开始，第 0 个参数是 wg 自己。
		for j := 1; j < method.Type.NumIn(); j++ {
			//In是返回第i个参数的类型，返回值是Type
			argv = append(argv, method.Type.In(j).Name())
		}
		for j := 0; j < method.Type.NumOut(); j++ {
			//Out是返回第i个返回值的类型，返回值是Type
			returns = append(returns, method.Type.Out(j).Name())
		}

		log.Printf("func (w *%s) %s(%s) %s",
			//Elem返回该interface的元素类型，返回值是Type
			typ.Elem().Name(),
			method.Name,
			strings.Join(argv, ","),
			strings.Join(returns, ","))
	}
	//s := []string{"foo", "bar", "baz"}
	//fmt.Println(s)
	//fmt.Println(strings.Join(s, ", "))
}

// Method代表一个方法
//type Method struct {
//	// Name是方法名。PkgPath是非导出字段的包路径，对导出字段该字段为""。
//	Name    string
//	PkgPath string
//	Type    Type  // 方法类型
//	Func    Value // 方法的值，什么叫做方法的值
//	Index   int   // 用于Type.Method的索引
//}

//Type代表一个接口（抽象类）
//type Type interface {
//	// Kind返回该接口的具体分类
//	Kind() Kind
//	// Name返回该类型在自身包内的类型名，如果是未命名类型会返回""
//	Name() string
//	// 返回类型的字符串表示。该字符串可能会使用短包名（如用base64代替"encoding/base64"）
//	// 也不保证每个类型的字符串表示不同。如果要比较两个类型是否相等，请直接用Type类型比较。
//	String() string
//	// 返回要保存一个该类型的值需要多少字节；类似unsafe.Sizeof
//	// 返回该类型的元素类型，如果该类型的Kind不是Array、Chan、Map、Ptr或Slice，会panic
//	Elem() Type
//	// 返回map类型的键的类型。如非映射类型将panic
//	Key() Type
//	// 返回func类型的参数个数，如果不是函数，将会panic
//	NumIn() int
//	// 返回func类型的第i个参数的类型，如非函数或者i不在[0, NumIn())内将会panic
//	In(i int) Type
//	// 返回func类型的返回值个数，如果不是函数，将会panic
//	NumOut() int
//	// 返回func类型的第i个返回值的类型，如非函数或者i不在[0, NumOut())内将会panic
//	Out(i int) Type
//	// 返回该类型的方法集中方法的数目
//	// 匿名字段的方法会被计算；主体类型的方法会屏蔽匿名字段的同名方法；
//	// 匿名字段导致的歧义方法会滤除
//	NumMethod() int
//	// 返回该类型方法集中的第i个方法，i不在[0, NumMethod())范围内时，将导致panic
//	// 对非接口类型T或*T，返回值的Type字段和Func字段描述方法的未绑定函数状态
//	// 对接口类型，返回值的Type字段描述方法的签名，Func字段为nil
//	Method(int) Method
//	// 根据方法名返回该类型方法集中的方法，使用一个布尔值说明是否发现该方法
//	// 对非接口类型T或*T，返回值的Type字段和Func字段描述方法的未绑定函数状态
//	// 对接口类型，返回值的Type字段描述方法的签名，Func字段为nil
//	MethodByName(string) (Method, bool)
//	// 内含隐藏或非导出方法
//}
