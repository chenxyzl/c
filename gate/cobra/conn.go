package cobra

import (
	"bufio"
	"golang.org/x/net/websocket"
	"io"
	"net"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/topfreegames/pitaya/logger"
)

const msgSizeBytes uint32 = 4

//连接类型
type ConnectType int32

const (
	ConnectType_TCP ConnectType = 0
	ConnectType_WS  ConnectType = 1
)

//IConn 通用连接
type IConn interface {
	Read(p []byte) (n int, err error)
	Write(b []byte) (n int, err error)
	SetReadDeadline(t time.Time) error
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	Close() error
}

type conn struct {
	//流量统计
	readMsgCount    uint32
	writeMsgCount   uint32
	readMsgSize     uint64
	writeMsgSize    uint64
	readMsgMaxSize  uint32
	writeMsgMaxSize uint32

	rwc        IConn
	remoteAddr string
	localAddr  string

	msgLength []byte

	broker       *Broker
	writeTimeOut time.Duration
	readTimeOut  time.Duration
	connectType  ConnectType
}

func newconn(cn IConn, bk *Broker, connectType ConnectType) *conn {
	c := &conn{
		rwc:        cn,
		remoteAddr: cn.RemoteAddr().String(),
		localAddr:  cn.LocalAddr().String(),
		msgLength:  make([]byte, msgSizeBytes),
		broker:     bk,
	}
	if c.broker.conf.WriteTimeOut > 0 {
		c.writeTimeOut = time.Duration(c.broker.conf.WriteTimeOut) * time.Second
	} else {
		c.writeTimeOut = WRITE_TIME_OUT
	}

	if c.broker.conf.ReadTimeOut > 0 {
		c.readTimeOut = time.Duration(c.broker.conf.ReadTimeOut) * time.Second
	} else {
		c.readTimeOut = READ_TIME_OUT
	}
	return c
}

func (this *conn) readLoop() {
	rbuf := bufio.NewReader(this.rwc)
	for {
		this.rwc.SetReadDeadline(time.Now().Add(this.readTimeOut))
		buf, err := this.read(rbuf)
		if err != nil {
			if err == io.EOF {
				logger.Log.Info("[conn] read EOF: %s %s %s",
					this.localAddr, this.remoteAddr, err)
			} else {
				logger.Log.Error("[conn] read error: %s %s %s",
					this.localAddr, this.remoteAddr, err)
			}
			this.broker.Stop()
			goto exit
		}
		this.broker.transmitOrProcessMsg(buf)
	}
exit:
	this.broker.wg.Done()
}

func (this *conn) read(r io.Reader) ([]byte, error) {
	_, err := io.ReadFull(r, this.msgLength)
	if err != nil {
		if err == io.EOF {
			logger.Log.Info("[conn] io read length EOF: %s %s %v",
				this.localAddr, this.remoteAddr, err)
		} else {
			logger.Log.Error("[conn] io read length error: %s %s %v",
				this.localAddr, this.remoteAddr, err)
		}
		return nil, err
	}

	//    [x][x][x][x][x][x][x][x]...
	//    |  (int32) || (binary)
	//    |  4-byte  || N-byte
	//    ------------------------...
	//        size       data
	msgSize := DecodeUint32(this.msgLength)
	if msgSize < PACK_HEAD_LEN ||
		msgSize > uint32(this.broker.conf.MaxReadMsgSize) {
		logger.Log.Error("[conn] pack length error: %s %s len: %d",
			this.localAddr, this.remoteAddr, msgSize)
		return nil, ReadOverflow
	}

	buf := make([]byte, msgSize-msgSizeBytes)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		logger.Log.Error("[conn] io read data error: %s %s %v",
			this.localAddr, this.remoteAddr, err)
		return nil, err
	}
	//统计接收的消息
	this.readMsgCount++
	this.readMsgSize += uint64(msgSize)
	if msgSize > this.readMsgMaxSize {
		this.readMsgMaxSize = msgSize
	}
	if this.readMsgCount&0xfff == 0 {
		logger.Log.Info("[conn] socket(%s-%s) read info: %d %d %d",
			this.localAddr, this.remoteAddr, this.readMsgCount, this.readMsgMaxSize, this.readMsgSize)
	}
	//TS.setRead(uint64(msgSize))
	return buf, nil
}

func (this *conn) wsRead() ([]byte, error) {
	_, err := this.rwc.Read(this.msgLength)
	if err != nil {
		if err == io.EOF {
			logger.Log.Info("[conn] io read length EOF: %s %s %v",
				this.localAddr, this.remoteAddr, err)
		} else {
			logger.Log.Error("[conn] io read length error: %s %s %v",
				this.localAddr, this.remoteAddr, err)
		}
		return nil, err
	}

	//    [x][x][x][x][x][x][x][x]...
	//    |  (int32) || (binary)
	//    |  4-byte  || N-byte
	//    ------------------------...
	//        size       data
	msgSize := DecodeUint32(this.msgLength)
	if msgSize < PACK_HEAD_LEN ||
		msgSize > uint32(this.broker.conf.MaxReadMsgSize) {
		logger.Log.Error("[conn] pack length error: %s %s len: %d",
			this.localAddr, this.remoteAddr, msgSize)
		return nil, ReadOverflow
	}

	buf := make([]byte, msgSize-msgSizeBytes)
	_, err = this.rwc.Read(buf)
	if err != nil {
		logger.Log.Error("[conn] io read data error: %s %s %v",
			this.localAddr, this.remoteAddr, err)
		return nil, err
	}
	//统计接收的消息
	this.readMsgCount++
	this.readMsgSize += uint64(msgSize)
	if msgSize > this.readMsgMaxSize {
		this.readMsgMaxSize = msgSize
	}
	if this.readMsgCount&0xfff == 0 {
		logger.Log.Info("[conn] socket(%s-%s) read info: %d %d %d",
			this.localAddr, this.remoteAddr, this.readMsgCount, this.readMsgMaxSize, this.readMsgSize)
	}
	//TS.setRead(uint64(msgSize))
	return buf, nil
}

func (this *conn) writeLoop() {
	max_size := this.broker.conf.MaxWriteMsgSize
	write_buff := make([]byte, max_size)
	head_buff := make([]byte, PACK_HEAD_LEN)
	data_buff := make([]byte, max_size-PACK_HEAD_LEN)
	for {
		select {
		case msg := <-this.broker.writeMsgQueue:
			length, data, err := Marshal(msg.PH, msg.Info, max_size, head_buff, data_buff)
			if err == nil {
				index := 0
				copy(write_buff, head_buff)
				copy(write_buff[PACK_HEAD_LEN:], data)
				index += length
				for more := true; more; {
					select {
					case msg := <-this.broker.writeMsgQueue:
						length, data, err := Marshal(msg.PH, msg.Info, max_size, head_buff, data_buff)
						if err == nil {
							if index+length <= max_size {
								copy(write_buff[index:], head_buff)
								copy(write_buff[index+PACK_HEAD_LEN:], data)
								index += length
							} else {
								if !this.Write(write_buff[:index]) {
									goto exit
								}
								index = 0
								copy(write_buff, head_buff)
								copy(write_buff[PACK_HEAD_LEN:], data)
								index += length
							}
						}
					case <-this.broker.CloseChan:
						goto exit
					default:
						more = false
					}
				}
				if !this.Write(write_buff[:index]) {
					goto exit
				}
			}
		case <-this.broker.CloseChan:
			goto exit
		}
	}
exit:
	this.broker.wg.Done()
}

func (this *conn) Write(msg []byte) bool {
	//this.rwc.SetWriteDeadline(time.Now().Add(this.writeTimeOut))
	if this.connectType == ConnectType_WS {
		this.rwc.(*websocket.Conn).PayloadType = websocket.BinaryFrame
	}
	n, err := this.rwc.Write(msg)
	//msg = nil
	if err != nil {
		logger.Log.Error("[conn] write error: %s %s %v",
			this.localAddr, this.remoteAddr, err)
		this.broker.Stop()
		return false
	}
	//统计发出的消息
	this.writeMsgCount++
	this.writeMsgSize += uint64(n)
	if uint32(n) > this.writeMsgMaxSize {
		this.writeMsgMaxSize = uint32(n)
	}
	if this.writeMsgCount&0xfff == 0 {
		logger.Log.Info("[conn] socket(%s-%s) write info: %d %d %d",
			this.localAddr, this.remoteAddr, this.writeMsgCount, this.writeMsgMaxSize, this.writeMsgSize)
	}
	//TS.setWrite(uint64(n))
	return true
}

func (this *conn) close() {
	this.broker = nil
}

func Marshal(ph *PackHead, msg interface{}, max_size int, head_buff, data_buff []byte) (int, []byte, error) {
	var data []byte
	switch v := msg.(type) {
	case []byte:
		data = v
	case proto.Message:
		if mdata, err := MarshalWithBytes(v, data_buff[0:0]); err == nil {
			data = mdata
		} else {
			logger.Log.Error("[Broker] proto marshal cmd: %d uid: %d error: %v",
				ph.Cmd, ph.Uid, err)
			return 0, nil, err
		}
	case nil:
		data = nil
	default:
		logger.Log.Error("[Broker] error msg type cmd: %d uid: %d",
			ph.Cmd, ph.Uid)
		return 0, nil, ErrorMsgType
	}

	length := len(data) + PACK_HEAD_LEN
	if length > max_size {
		logger.Log.Error("[Broker] write msg size overflow cmd: %d uid: %d length: %d",
			ph.Cmd, ph.Uid, length)
		return 0, nil, WriteOverflow
	}

	ph.Length = uint32(length)
	logger.Log.Debug("[Broker] write head %v", ph)
	EncodePackHead(head_buff, ph)
	return length, data, nil
}
