package cobra

import (
	"golang.org/x/net/websocket"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/topfreegames/pitaya/logger"
)

type Sessioner interface {
	Init(*Broker)              //初始化操作，比如心跳的设置...
	Process(*PackHead, []byte) //处理消息
	Close()                    //消除所有对Sessioner的引用,心跳...
}

const (
	StateInit = iota
	StateDisconnected
	StateConnected
)

type Broker struct {
	cn      *conn
	session Sessioner
	conf    *Config

	ReadMsgQueue  chan []byte
	writeMsgQueue chan *Message

	state     int32
	CloseChan chan struct{}

	wg sync.WaitGroup
}

func newBroker(se Sessioner, cf *Config) *Broker {
	return &Broker{
		session: se,
		conf:    cf,
	}
}

func (this *Broker) LocalAddr() string { return this.cn.localAddr }

func (this *Broker) RemoteAddr() string { return this.cn.remoteAddr }

func (this *Broker) State() int32 {
	return atomic.LoadInt32(&this.state)
}

func (this *Broker) Connect(timeout time.Duration) bool {
	rw, err := net.DialTimeout("tcp", this.conf.Address, timeout)
	if err != nil {
		logger.Log.Error("[Broker] Connect Error: %v", err)
		return false
	}
	if !this.serve(rw, ConnectType_TCP) {
		rw.Close()
		return false
	}
	return true
}

func (this *Broker) WsConnect() bool {
	rw, err := websocket.Dial(this.conf.Address, "", "")
	if err != nil {
		logger.Log.Error("[Broker] Connect Error: %v", err)
		return false
	}
	if !this.serve(rw, ConnectType_WS) {
		rw.Close()
		return false
	}
	return true
}

func (this *Broker) serve(rwc IConn, connectType ConnectType) bool {
	if !atomic.CompareAndSwapInt32(&this.state, StateInit, StateConnected) {
		return false
	}

	this.cn = newconn(rwc, this, connectType)
	this.CloseChan = make(chan struct{})
	this.writeMsgQueue = make(chan *Message, this.conf.WriteMsgQueueSize)
	if this.conf.ReadMsgQueueSize > 0 {
		this.ReadMsgQueue = make(chan []byte, this.conf.ReadMsgQueueSize)
	}

	this.wg.Add(2)
	this.session.Init(this)

	go this.cn.writeLoop()

	if connectType == ConnectType_TCP {
		go this.cn.readLoop()
	} else if connectType == ConnectType_WS {
		this.cn.wsReadLoop()
	}

	return true
}

func (this *conn) wsReadLoop() {
	for {
		this.rwc.SetReadDeadline(time.Now().Add(this.readTimeOut))
		buf, err := this.wsRead()
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

func (this *Broker) AddWaitGroup() {
	this.wg.Add(1)
}

func (this *Broker) DecWaitGroup() {
	this.wg.Done()
}

func (this *Broker) transmitOrProcessMsg(buf []byte) {
	if this.conf.ReadMsgQueueSize > 0 {
		select {
		case this.ReadMsgQueue <- buf:
		case <-this.CloseChan:
		}
	} else {
		this.session.Process(GetInputMsgPackHead(buf), buf[12:])
	}
}

func GetInputMsgPackHead(buf []byte) *PackHead {
	return &PackHead{
		Length: uint32(len(buf) + 4),
		Cmd:    DecodeUint32(buf[0:]),
		Uid:    DecodeUint64(buf[4:]),
	}
}

func (this *Broker) Stop() {
	if !atomic.CompareAndSwapInt32(&this.state, StateConnected, StateDisconnected) {
		return
	}
	close(this.CloseChan)
	this.cn.rwc.Close()
	go func() {
		this.wg.Wait()
		this.cn.close()
		this.session.Close()
		logger.Log.Info("[Broker] closed, addr: (%s %s)", this.cn.localAddr, this.cn.remoteAddr)
		//this.cn = nil
		this.session = nil
		atomic.StoreInt32(&this.state, StateInit)
	}()
}

type Message struct {
	PH   *PackHead
	Info interface{}
}

// 注意：上层逻辑每次使用Write时，ph和msg需要重新分配，禁止共用，
// 防止并发异常（例如writeLoop时Marshal会修改ph.Length等）
func (this *Broker) Write(ph *PackHead, msg interface{}) bool {
	tmp := new(PackHead)
	select {
	case this.writeMsgQueue <- &Message{tmp.copy(ph), msg}:
		return true
	case <-this.CloseChan:
		logger.Log.Error("session close: %v %v", ph, msg)
		return false
	}
	/*
		if data, err := this.Marshal(ph, msg); err == nil {
			mq_len := len(this.writeMsgQueue)
			mq_cap := cap(this.writeMsgQueue)
			if mq_len > int(HIGH_WATER_MARK_SCALE*float64(mq_cap)) {
				logger.Log.Warn("[Broker] writeMsgQueue is HighWaterMark, len: %d cap: %d addr: (%s %s)",
					mq_len, mq_cap, this.cn.localAddr, this.cn.remoteAddr)
			}
			select {
			case this.writeMsgQueue <- data:
			case <-this.CloseChan:
			}
		}
	*/
}

func (this *Broker) Marshal(ph *PackHead, msg interface{}) ([]byte, error) {
	var data []byte
	switch v := msg.(type) {
	case []byte:
		data = make([]byte, len(v)+PACK_HEAD_LEN)
		copy(data[PACK_HEAD_LEN:], v)
	case proto.Message:
		data = make([]byte, PACK_HEAD_LEN, 64)
		if mdata, err := MarshalWithBytes(v, data); err == nil {
			data = mdata
		} else {
			logger.Log.Error("[Broker] proto marshal cmd: %d uid: %d error: %v",
				ph.Cmd, ph.Uid, err)
			return nil, err
		}
	case nil:
		data = make([]byte, PACK_HEAD_LEN)
	default:
		logger.Log.Error("[Broker] error msg type cmd: %d uid: %d",
			ph.Cmd, ph.Uid)
		return nil, ErrorMsgType
	}

	length := len(data)
	if length > this.conf.MaxWriteMsgSize {
		logger.Log.Error("[Broker] write msg size overflow cmd: %d uid: %d length: %d",
			ph.Cmd, ph.Uid, length)
		return nil, WriteOverflow
	}

	ph.Length = uint32(length)
	logger.Log.Debug("[Broker] write head %v", ph)
	EncodePackHead(data, ph)
	return data, nil
}

func MarshalWithBytes(pb proto.Message, data []byte) ([]byte, error) {
	p := proto.NewBuffer(data)
	err := p.Marshal(pb)
	if err != nil {
		// Ignore unset required fields.
		_, ok := err.(*proto.RequiredNotSetError)
		if !ok {
			return nil, err
		}
	}
	return p.Bytes(), err
}
