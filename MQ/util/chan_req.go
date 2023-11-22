package util

// 定义在channel中传输的内容

type ChanReq struct {
	Variable interface{}
	Retchan  chan interface{}
}

type ChanRet struct {
	Err      error
	Variable interface{}
}
