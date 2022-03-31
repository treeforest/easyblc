package pb

// SendTxReq 发送交易请求
type SendTxReq struct {
	Tx []byte
}

// SendTxReply 发送交易响应
type SendTxReply struct {
	Status bool
}

// QueryReq 查询请求
type QueryReq struct {
	BloomData []byte
}

// QueryReply 查询响应
type QueryReply struct {
	Body interface{} // 地址 =》 余额
}
