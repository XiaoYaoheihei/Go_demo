package util

import (
	"crypto/rand"
	"fmt"
	"io"
	"log"
)

var UuidChan = make(chan []byte, 1000)

// 工厂模式不断生成uuid
func UuidFactory() {
	for {
		UuidChan <- uuid()
	}
}

func uuid() []byte {
	tmp := make([]byte, 16)
	_, err := io.ReadFull(rand.Reader, tmp)
	if err != nil {
		log.Fatal(err)
	}
	return tmp
}

func UuidTostring(b []byte) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
