package sess

import (
	"github.com/golang/protobuf/proto"
	"c/framework/actor"
	"c/gate/cobra"
	"c/gate/pack"
	"c/protos/out/cg"

	"github.com/topfreegames/pitaya/logger"
)

type STAT uint32

const (
	STAT_NEW  STAT = 1
	STAT_AUTH STAT = 2
)

// logicSession 连接
type logicSession struct {
	act       *actor.Actor
	broker    *cobra.Broker
	sessionID uint64
	lgcMgr    *LogicManager
	sids      []uint64 //服务器id列表
	clients   map[uint64]*ClientSession
}

func newServerSession(ma *LogicManager, cfg *actor.Config, sessionID uint64) *logicSession {
	return &logicSession{
		lgcMgr:    ma,
		sessionID: sessionID,
		act:       actor.NewActor(cfg),
		sids:      make([]uint64, 0),
		clients:   make(map[uint64]*ClientSession),
	}
}

// Run
func (c *logicSession) Run() {
	c.act.Run(c)
}

// SessionID
func (c *logicSession) SessionID() uint64 {
	return c.sessionID
}

// Init
func (c *logicSession) Init(broker *cobra.Broker) {
	c.broker = broker
	c.lgcMgr.addClient(c)

	//返回logic cg::MSG_C2G_KeepAlive
	ph := &cobra.PackHead{
		Cmd: uint32(cg.ID_MSG_C2G_SayHi),
		Uid: c.SessionID(),
	}
	c.Write(ph, &cg.C2G_SayHi{})
	//sayHi
	logger.Log.Debug("[logicSession] init: sessionID=%d local=%s remote=%s", c.sessionID, c.broker.LocalAddr(), c.broker.RemoteAddr())
}

// Close
func (c *logicSession) Close() {
	c.lgcMgr.deleteClient(c)
	logger.Log.Debug("[logicSession] close: sessionID=%d local=%s remote=%s", c.sessionID, c.broker.LocalAddr(), c.broker.RemoteAddr())
}

// Write
func (c *logicSession) Write(ph *cobra.PackHead, msg interface{}) {
	c.broker.Write(ph, msg)
}

// Process
func (c *logicSession) Process(ph *cobra.PackHead, data []byte) {
	msg := &pack.Message{c.sessionID, ph, data}
	logger.Log.Fine("[logicSession] Process sessionID=%d msg: %d", c.sessionID, ph.Cmd)
	c.Parse(msg)
}

func (c *logicSession) stop() {
	c.act.Stop()
	logger.Log.Warn("[logicSession] stop")
}

func (c *logicSession) Stop() {
	c.broker.Stop()
	logger.Log.Warn("[logicSession] Stop")
}

func (c *logicSession) wait() {
	c.act.Wait()
	if c.broker != nil {
		c.broker.Stop()
	}
	logger.Log.Warn("[logicSession] wait")
}

func (c *logicSession) keepAlive() {
	//返回logic cg::MSG_C2G_KeepAlive
	ph := &cobra.PackHead{
		Cmd: uint32(cg.ID_MSG_C2G_KeepAlive),
	}
	c.Write(ph, &cg.C2G_KeepAlive{})
	logger.Log.Info("[logicSession]keepAlive id[%d] heart beat", c.SessionID())
}

func (c *logicSession) registerServerId(msg *pack.Message) {
	in := &cg.G2C_SayHi{}
	err := proto.Unmarshal(msg.Data, in)
	if err != nil {
		logger.Log.Error("[logicSession]registerServerId unmarshal err=[%v]", err)
		return
	}

	ok := c.lgcMgr.bindClient(c.sessionID, in.GetCurrent())
	if !ok {
		logger.Log.Error("[logicSession]registerServerId failed,serverId repeated session[%d] mid=[%v] aid=[%v]", c.SessionID(), in.GetId(), in.GetCurrent())
		c.stop()
		return
	}

	//返回logic cg::C2G_SayHi
	logger.Log.Info("[logicSession]registerServerId session[%d] mid=[%v] aid=[%v]", c.SessionID(), in.GetId(), in.GetCurrent())
}

func (c *logicSession) broadcast(msg *pack.Message) {
	in := &cg.G2C_Broadcast{}
	err := proto.Unmarshal(msg.Data, in)
	if err != nil {
		logger.Log.Error("[logicSession]broadcast unmarshal err=[%v]", err)
		return
	}

	ph := &cobra.PackHead{
		Cmd: uint32(in.GetCmd()),
	}

	if len(in.GetIds()) != 0 { //指定uid广播
		for _, uid := range in.GetIds() {
			if client, ok := c.clients[uid]; ok {
				client.Write(ph, in.GetInfo())
			}
		}
	} else { //全局广播
		for _, client := range c.clients {
			client.Write(ph, in.GetInfo())
		}
	}

	logger.Log.Info("[logicSession]broadcast sid[%d] uids=[%v]", c.SessionID(), in.GetIds())
}

func (c *logicSession) AddClientSession(clientSession *ClientSession) {
	c.clients[clientSession.SessionID()] = clientSession
}

func (c *logicSession) RemoveClientSession(clientSession *ClientSession) {
	delete(c.clients, clientSession.SessionID())
}

func (c *logicSession) Parse(msg *pack.Message) bool {
	switch cg.ID(msg.PH.Cmd) {
	case cg.ID_MSG_G2C_KeepAlive:
		c.keepAlive()
		return true
	case cg.ID_MSG_G2C_SayHi:
		c.registerServerId(msg)
		return true
	case cg.ID_MSG_G2C_Broadcast:
		c.broadcast(msg)
		return true
	case cg.ID_MSG_G2C_Login:
		fallthrough
	case cg.ID_MSG_G2C_Create:
		fallthrough
	case cg.ID_MSG_G2C_Offline:
		client, ok := c.clients[msg.PH.Uid]
		if !ok {
			logger.Log.Error("[logicSession]Dispatcher no found client offline, uniq_id: %v cmd: %v", msg.PH.Uid, msg.PH.Cmd)
			return false
		}
		return client.ProcessLogicMsg(msg)
	default:
		client, ok := c.clients[msg.PH.Uid]
		if !ok {
			logger.Log.Error("[logicSession]Dispatcher no found client default, uniq_id: %v cmd: %v", msg.PH.Uid, msg.PH.Cmd)
			return false
		}
		client.Write(msg.PH, msg.Data)
		return true
	}
	return true
}
