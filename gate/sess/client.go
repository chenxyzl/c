package sess

import (
	"github.com/golang/protobuf/proto"
	"time"
	"c/framework/actor"
	"c/gate/cobra"
	"c/gate/pack"
	"c/protos/out/cg"

	"github.com/topfreegames/pitaya/logger"
)

type SessionState uint32

const (
	kNone       SessionState = 0
	kLogin      SessionState = 1
	kCreate     SessionState = 2
	kWaitCreate SessionState = 3
	kNormal     SessionState = 4
	kOffline    SessionState = 5
	kClear      SessionState = 6
	kEnterScene SessionState = 7
	kLoginQueue SessionState = 8
)

// ClientSession 连接
type ClientSession struct {
	act       *actor.Actor
	broker    *cobra.Broker
	sessionID uint64
	uid       uint64
	sid       uint64
	cltMgr    *ClientManager
	stat      SessionState
	qpsTime   int64
	qpsNo     uint64
}

func newClientSession(ma *ClientManager, cfg *actor.Config, sessionID uint64) *ClientSession {
	return &ClientSession{
		cltMgr:    ma,
		sessionID: sessionID,
		uid:       sessionID, //初始uid和sessionID一样，登陆成功后替换
		act:       actor.NewActor(cfg),
		stat:      kNone,
	}
}

// Run
func (c *ClientSession) Run() {
	c.act.Run(c)
}

// SessionID
func (c *ClientSession) SessionID() uint64 {
	return c.sessionID
}

// Uid
func (c *ClientSession) Uid() uint64 {
	return c.uid
}

// Init
func (c *ClientSession) Init(broker *cobra.Broker) {
	c.broker = broker
	c.cltMgr.addClient(c)
	logger.Log.Debug("[ClientSession] init: sessionID=%d local=%s remote=%s", c.sessionID, c.broker.LocalAddr(), c.broker.RemoteAddr())
}

// Close
func (c *ClientSession) Close() {
	c.cltMgr.deleteClient(c)
	logic := lgcMgr.GetSessionBySid(c.sid)
	if logic != nil {
		logic.RemoveClientSession(c)
	}

	if c.stat != kNone || c.stat != kClear {
		header := &cobra.PackHead{
			Cmd: uint32(cg.ID_MSG_C2G_Offline),
			Uid: c.SessionID(),
		}
		c.Write2Logic(header, nil)
	}

	logger.Log.Debug("[ClientSession] close: sessionID=%d local=%s remote=%s", c.sessionID, c.broker.LocalAddr(), c.broker.RemoteAddr())
}

// Write
func (c *ClientSession) Write(ph *cobra.PackHead, msg interface{}) {
	ph.Uid = c.Uid()
	c.broker.Write(ph, msg)
}

func (c *ClientSession) Write2Logic(ph *cobra.PackHead, msg interface{}) bool {
	ph.Uid = c.Uid()
	logic := lgcMgr.GetSessionBySid(c.sid)
	if logic == nil {
		logger.Log.Error("[ClientSession]Write2Logic client send logic error, id: %d unique_id: %d server_id: %d cmd: %d", c.Uid(), c.SessionID(), c.sid, ph.Cmd)
		return false
	}

	logic.Write(ph, msg)
	return true
}

// Process
func (c *ClientSession) Process(ph *cobra.PackHead, data []byte) {
	msg := &pack.Message{c.sessionID, ph, data}
	logger.Log.Fine("[ClientSession] Process sessionID=%d msg: %d", c.sessionID, ph.Cmd)
	c.ProcessClientMsg(msg)
}

func (c *ClientSession) stop() {
	c.act.Stop()
}

//Stop Stop
func (c *ClientSession) Stop() {
	c.broker.Stop()
}

func (c *ClientSession) wait() {
	c.act.Wait()
	if c.broker != nil {
		c.broker.Stop()
	}
}

func (c *ClientSession) keepAlive() bool {
	header := &cobra.PackHead{
		Cmd: uint32(cg.ID_MSG_G2C_KeepAlive),
	}
	c.Write(header, nil)
	return true
}

func (c *ClientSession) registerServerId(msg *pack.Message) {
	//返回logic cg::C2G_SayHi
}

func (c *ClientSession) login(header *cobra.PackHead, data []byte) bool {
	//返回logic cg::C2G_SayHi
	in := &cg.C2G_Login{}
	err := proto.Unmarshal(data, in)
	if err != nil {
		logger.Log.Error("[ClientSession]login err=[%v]", err)
		return false
	}
	c.sid = in.GetServerId()
	logic := lgcMgr.GetSessionBySid(c.sid)
	if logic == nil {
		header := &cobra.PackHead{
			Cmd: uint32(cg.ID_MSG_G2C_Login),
		}
		out := &cg.G2C_Login{
			Ret: proto.Uint32(uint32(cg.RET_RET_SERVER_MAINTAIN)),
		}
		c.Write(header, out)
		return false
	}

	c.stat = kLogin
	logic.AddClientSession(c)
	c.Write2Logic(header, data)
	return true
}

func (c *ClientSession) create(header *cobra.PackHead, data []byte) bool {
	c.stat = kCreate
	c.Write2Logic(header, data)
	return true
}

func (c *ClientSession) ProcessLogicMsg(msg *pack.Message) bool {
	ok := c.processLogicMsg(msg)
	if !ok {
		c.Stop()
		logger.Log.Error("[ClientSession]ProcessLogicMsg false; close session=[%d], uid=[%d]", c.SessionID(), c.Uid())
	}
	return ok
}

func (c *ClientSession) finishOffline(msg *pack.Message) bool {
	logger.Log.Error("[ClientSession]finishOffline session=[%v], uid=[%v]", c.SessionID(), msg.PH.Uid)
	return false
}

func (c *ClientSession) processLogin(msg *pack.Message) bool {
	in := &cg.G2C_Login{}
	if err := proto.Unmarshal(msg.Data, in); err != nil {
		logger.Log.Error("[ClientSession]processLogin session=[%v], uid=[%v], err=[%v]", c.SessionID(), msg.PH.Uid, err)
		return false
	}
	switch cg.RET(in.GetRet()) {
	case cg.RET_RET_OK:
		c.stat = kNormal
		c.uid = in.GetUid()
	case cg.RET_RET_USER_NOT_EXIST:
		c.stat = kWaitCreate
	case cg.RET_RET_ENTER_QUEUE:
		c.stat = kLoginQueue
	case cg.RET_RET_LOGIN_REPEAT:
		c.stat = kClear
	default:
		c.stat = kOffline
	}
	//发送
	c.Write(msg.PH, msg.Data)

	if c.stat == kOffline {
		return false
	}
	return true
}

func (c *ClientSession) finishCreate(msg *pack.Message) bool {
	in := &cg.G2C_Create{}
	if err := proto.Unmarshal(msg.Data, in); err != nil {
		logger.Log.Error("[ClientSession]finishCreate session=[%v], uid=[%v], err=[%v]", c.SessionID(), msg.PH.Uid, err)
		return false
	}
	switch cg.RET(in.GetRet()) {
	case cg.RET_RET_OK:
		c.stat = kNormal
		c.uid = in.GetUid()
	case cg.RET_RET_USER_NAME_REPEAT:
		c.stat = kWaitCreate
	default:
		c.stat = kOffline
	}
	//发送
	c.Write(msg.PH, msg.Data)

	if c.stat == kOffline {
		return false
	}
	return false
}

func (c *ClientSession) processLogicMsg(msg *pack.Message) bool {
	if msg.PH.Cmd == uint32(cg.ID_MSG_G2C_Offline) {
		return c.finishOffline(msg)
	}

	switch c.stat {
	case kLogin:
		fallthrough
	case kLoginQueue:
		if msg.PH.Cmd == uint32(cg.ID_MSG_G2C_Login) {
			return c.processLogin(msg)
		}
	case kCreate:
		if msg.PH.Cmd == uint32(cg.ID_MSG_G2C_Create) {
			return c.finishCreate(msg)
		}
	}

	logger.Log.Error("[ClientSession]processLogicMsg exceptional msg, state: %d uniq_id: %d cmd: %d", c.stat, c.SessionID(), msg.PH.Cmd)
	return false
}

func (c *ClientSession) ProcessClientMsg(msg *pack.Message) bool {
	ok := c.processClientMsg(msg)
	if !ok {
		c.Stop()
		logger.Log.Error("[ClientSession]ProcessClientMsg false; close session=[%d], uid=[%d]", c.SessionID(), c.Uid())
	}
	return ok
}

func (c *ClientSession) processClientMsg(msg *pack.Message) bool {
	now := time.Now().Unix()
	if c.qpsTime != now {
		c.qpsTime = now
		c.qpsNo = 1
	} else {
		c.qpsNo++
		if c.qpsNo > c.cltMgr.GetMaxQps() {
			logger.Log.Error("[ClientSession]Parse  client session qps[%v] is too fast, uid: %v uniq_id: %v", c.qpsNo, c.Uid(), c.SessionID())
			return false
		}
	}

	switch c.stat {
	case kNone:
		//没登陆也可以保持连接
		if msg.PH.Cmd == uint32(cg.ID_MSG_C2G_KeepAlive) {
			return c.keepAlive()
		}

		if msg.PH.Cmd != uint32(cg.ID_MSG_C2G_Login) {
			logger.Log.Error("[ClientSession]Parse  client first msg not login, id: %v uniq_id: %v", c.Uid(), c.SessionID())
			return false
		}
		msg.PH.Uid = c.SessionID()
		return c.login(msg.PH, msg.Data)
	case kLogin:
		fallthrough
	case kCreate:
		fallthrough
	case kLoginQueue:
		if msg.PH.Cmd != uint32(cg.ID_MSG_C2G_KeepAlive) {
			logger.Log.Error("[ClientSession]Parse client exceptional msg, state: %d uniq_id: %d cmd: %d", c.stat, c.SessionID(), msg.PH.Cmd)
			return false
		}
		return c.keepAlive()
	case kWaitCreate:
		if msg.PH.Cmd == uint32(cg.ID_MSG_C2G_KeepAlive) {
			return c.keepAlive()
		}
		if msg.PH.Cmd == uint32(cg.ID_MSG_C2G_Create) {
			msg.PH.Uid = c.SessionID()
			return c.create(msg.PH, msg.Data)
		}
		logger.Log.Error("[ClientSession]Parse client wait create, uniq_id: %d cmd: %d", c.SessionID(), msg.PH.Cmd)
		return false
	case kNormal:
		if msg.PH.Cmd == uint32(cg.ID_MSG_C2G_KeepAlive) {
			return c.keepAlive()
		}
		if msg.PH.Cmd <= uint32(cg.ID_MSG_LOGIC_MIN) || msg.PH.Cmd >= uint32(cg.ID_MSG_LOGIC_MAX) || msg.PH.Cmd == uint32(cg.ID_MSG_C2G_Create) || msg.PH.Cmd == uint32(cg.ID_MSG_C2G_Login) {
			logger.Log.Error("[ClientSession]Parse client(kNormal) send wrong type packet, uniq_id: %d cmd: %d", c.SessionID(), msg.PH.Cmd)
			return false
		}
		return c.Write2Logic(msg.PH, msg.Data)
	case kOffline:
		logger.Log.Error("[ClientSession]Parse client offline, uniq_id: %d cmd: %d", c.SessionID(), msg.PH.Cmd)
		return false
	default:
		logger.Log.Error("[ClientSession]Parse client session no found state msg, state: %d uniq_id: %d server_id: %d user_id: %d hid: %d cmd: %d", c.stat, c.SessionID(), c.sid, c.Uid(), msg.PH.Uid, msg.PH.Cmd)
		return false
	}
	return true
}
