package utils

import (
	"crypto/rand"
	"fmt"
	"io"
	"log"
)

// 生成消息的唯一标识符
func Uuid() []byte {
	tmp := make([]byte, 16)
	//ReadFull函数是用来从一个io.Reader对象中读取指定大小的数据块的函数。
	_, err := io.ReadFull(rand.Reader, tmp)
	if err != nil {
		log.Fatal(err)
	}
	return tmp
}

// 将Uuid转化成相应的string类型数据
func UuidTostring(b []byte) string {
	return fmt.Sprintf("%x-%x-%x-%x", b[:4], b[4:8], b[8:12], b[12:])
}
