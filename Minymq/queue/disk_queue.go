// 实现一个持久化队列保证消息不丢失
package queue

import (
	"Minymq/utils"
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
)

// 单个文件的大小是100MB
const maxFileSize = 1024 * 1024 * 100

type DiskQueue struct {
	//读写的文件名以及他们的位置信息
	name     string
	readPos  int64
	writePos int64
	//读文件个数
	readFileNum int64
	//写文家个数
	writeFileNum int64

	readFile  *os.File
	writeFile *os.File
	// readChan是为了确保当真正有读取后台队列需求
	readChan chan struct{}
	//发送消息管道--持久化使用
	inChan chan utils.ChanReq
	//读取消息管道--从持久化中读取
	outChan chan utils.ChanRet
	//退出管道
	exitChan chan utils.ChanReq
}

func NewDiskQueue(name string) *DiskQueue {
	diskqueue := &DiskQueue{
		name:     name,
		readChan: make(chan struct{}),
		inChan:   make(chan utils.ChanReq),
		outChan:  make(chan utils.ChanRet),
		exitChan: make(chan utils.ChanReq),
	}

	if _, err := os.Stat(diskqueue.metaDataFileName()); err == nil {
		err = diskqueue.retrieveMetaData()
		if err != nil {
			log.Printf("WARNING: failed to retrieveMetaData() - %s", err.Error())
		}
	}

	go diskqueue.Router()

	return diskqueue
}

func (d *DiskQueue) Close() error {
	d.exitChan <- utils.ChanReq{
		Variable: true,
	}

}

func (d *DiskQueue) metaDataFileName() string {
	return fmt.Sprintf("%s.diskqueue.meta.data", d.name)
}
func (d *DiskQueue) fileName(fileNum int64) string {
	return fmt.Sprintf("%s.diskqueue.%06d.data", d.name, fileNum)
}
func (d *DiskQueue) hasDataToRead() bool {
	return (d.writeFileNum > d.readFileNum) || (d.writePos > d.readPos)
}

// 读取消息
func (d *DiskQueue) Get() ([]byte, error) {
	ret := <-d.outChan
	return ret.Variable.([]byte), ret.Err
}

// 发送消息
func (d *DiskQueue) Put(bytes []byte) error {
	errChan := make(chan interface{})
	d.inChan <- utils.ChanReq{
		Variable: bytes,
	}
}

// readChan 是为了确保当真正有读取后台队列需求的时候才往 outChan 发送数据
func (d *DiskQueue) ReadReadyChan() chan struct{} {
	return d.readChan
}

// 持久化元数据
func (d *DiskQueue) persistMetaData() (err error) {
	metaFileName := d.metaDataFileName()
	tmpFileName := metaFileName + ".tmp"
	fd, err := os.OpenFile(tmpFileName, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return
	}
	//将相应的信息写入到文件当中去
	_, err = fmt.Fprintf(fd, "%d,%d\n%d,%d\n", d.readFileNum, d.readPos, d.writeFileNum, d.writePos)
	if err != nil {
		fd.Close()
		return
	}
	fd.Close()
	log.Printf("disk: persisted meta data for (%s) - readFileNum=%d writeFileNum=%d readPos=%d writePos=%d",
		d.name, d.readFileNum, d.writeFileNum, d.readPos, d.writePos)

	return os.Rename(tmpFileName, metaFileName)
}

// 恢复元数据
func (d *DiskQueue) retrieveMetaData() (err error) {
	metaFileName := d.metaDataFileName()
	fd, err := os.OpenFile(metaFileName, os.O_RDONLY, 0600)
	if err != nil {
		return
	}
	defer fd.Close()
	//从文件中读取相应的数据
	_, err = fmt.Fscanf(fd, "%d,%d\n%d,%d\n", &d.readFileNum, &d.readPos, &d.writeFileNum, &d.writePos)
	if err != nil {
		return
	}
	log.Printf("disk: retrieved meta data for (%s) - readFileNum=%d writeFileNum=%d readPos=%d writePos=%d",
		d.name, d.readFileNum, d.writeFileNum, d.readPos, d.writePos)

	return nil
}

// 从文件读取消息
func (d *DiskQueue) ReadOne() ([]byte, error) {
	var (
		err     error
		msgSize int32
	)
	//已经读取完成
	if d.readPos > maxFileSize {
		d.readFileNum++
		d.readPos = 0
		d.readFile.Close()
		d.readFile = nil
		//持久化信息
		if err = d.persistMetaData(); err != nil {
			return nil, err
		}
	}
	//读取的文件句柄为空
	if d.readFile == nil {
		//给文件句柄赋值
		d.readFile, err = os.OpenFile(d.fileName(d.readFileNum), os.O_RDONLY, 0600)
		if err != nil {
			return nil, err
		}
		//检查文件内容
		if d.readPos > 0 {
			_, err = d.readFile.Seek(d.readPos, 0)
			if err != nil {
				return nil, err
			}
		}
	}
	err = binary.Read(d.readFile, binary.BigEndian, &msgSize)
	if err != nil {
		d.readFile.Close()
		d.readFile = nil
		return nil, err
	}
	//读取缓冲区
	readBuf := make([]byte, msgSize)
	_, err = d.readFile.Read(readBuf)
	if err != nil {
		return nil, err
	}
	d.readPos += int64(msgSize + 4)
	return readBuf, nil
}

// 向文件写消息
func (d *DiskQueue) WriteOne(msg []byte) (err error) {
	var buf bytes.Buffer
	if d.writePos > maxFileSize {
		d.writeFileNum++
		d.writePos = 0
		d.writeFile.Close()
		d.writeFile = nil
		if err = d.persistMetaData(); err != nil {
			return
		}
	}

	if d.writeFile == nil {
		d.writeFile, err = os.OpenFile(d.fileName(d.writeFileNum), os.O_RDWR|os.O_CREATE, 0600)
		if err != nil {
			return
		}
		if d.writePos > 0 {
			_, err = d.writeFile.Seek(d.writePos, 0)
			if err != nil {
				return
			}
		}
	}

	dataLen := len(msg)
	err = binary.Write(&buf, binary.BigEndian, dataLen)
	if err != nil {
		return
	}

	_, err = buf.Write(msg)
	if err != nil {
		return
	}

	_, err = d.writeFile.Write(buf.Bytes())
	if err != nil {
		d.writeFile.Close()
		d.writeFile = nil
		return
	}
	d.writePos += int64(dataLen + 4)
	return
}

// 事件循环
func (d *DiskQueue) Router() {
	for {
		//有数据要读
		if d.hasDataToRead() {
			select {
			//为了仅在我们真正需要消息的时候读取
			case d.readChan <- struct{}{}:
				msg, err := d.ReadOne()
				d.outChan <- utils.ChanRet{
					Err:      err,
					Variable: msg,
				}
			//有消息需要持久化
			case writeRequest := <-d.inChan:
				_ = d.WriteOne(writeRequest.Variable.([]byte))
			case <-d.exitChan:
				if d.readFile != nil {
					d.readFile.Close()
				}
				if d.writeFile != nil {
					d.writeFile.Close()
				}
				//closeReq = d.persistMetaData()
				return
			}
		} else {
			select {
			case writeReq := <-d.inChan:
				_ = d.WriteOne(writeReq.Variable.([]byte))
			case <-d.exitChan:
				if d.readFile != nil {
					d.readFile.Close()
				}
				if d.writeFile != nil {
					d.writeFile.Close()
				}
				//closeReq = d.persistMetaData()
				return
			}
		}
	}
}
