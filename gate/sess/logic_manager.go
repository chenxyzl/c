package sess

import (
	"sync"
	"c/gate/cobra"
	"c/gate/global"
	"c/gate/pack"

	"github.com/topfreegames/pitaya/logger"
	"c/framework/actor"
	"c/framework/util"
)

// LogicManager session管理器
type LogicManager struct {
	sessionId   uint64
	mtx         sync.Mutex
	gm          *util.GroupManager
	cfg         *cobra.Config
	msgQ        chan *pack.Message
	lgcSessions sync.Map //logic的链接维护
	sids        sync.Map //logic sid和session的映射关系
}

var lgcMgr *LogicManager

// NewLogicManager 构造器
func NewLogicManager(cfg *cobra.Config, msgQueueSize int) *LogicManager {
	lgcMgr = &LogicManager{
		cfg: cfg,
		gm:  util.NewGroupManager(),
	}
	return lgcMgr
}

// WatchChan
func (mgr *LogicManager) WatchChan() <-chan struct{} {
	return mgr.gm.Chan()
}

// Run
func (mgr *LogicManager) GoRun() {
	//mgr.gm.Add(1)
	go func() {
		cobra.TCPServe(mgr, mgr.cfg)
		//mgr.gm.Done()
	}()
}

// Close
func (mgr *LogicManager) Close() {
	//关闭watch
	mgr.gm.Close()
	mgr.gm.Wait()
	mgr.lgcSessions.Range(func(key, value interface{}) bool {
		lgc := value.(*logicSession)
		lgc.stop()
		lgc.wait()
		mgr.lgcSessions.Delete(key)
		return true
	})
	logger.Log.Warn("[LogicManager] close...")
}

// NewSession 创建session回掉
func (mgr *LogicManager) NewSession() cobra.Sessioner {
	var logic *logicSession
	mgr.mtx.Lock()

	config := &actor.Config{
		Kind:      1,
		KindName:  "logic_actor",
		MsgQSize:  mgr.cfg.MaxReadMsgSize,
		Rate:      1000, //Actor ticker 频率(单位millsecond)
		TimerSize: 8,
	}
	logic = newServerSession(mgr, config, global.GenUUID())

	mgr.mtx.Unlock()
	if logic != nil {
		go logic.Run()
	}

	logger.Log.Info("[LogicManager] NewSession session=[%d]", logic.SessionID())

	return logic
}

// GetSession 获得Session
func (mgr *LogicManager) GetSession(sessionID uint64) *logicSession {
	client, ok := mgr.lgcSessions.Load(sessionID)
	if !ok {
		return nil
	}
	return client.(*logicSession)
}

func (mgr *LogicManager) GetSessionBySid(sid uint64) *logicSession {
	sessionId, ok := mgr.sids.Load(sid)
	if !ok {
		return nil
	}
	session, ok := mgr.lgcSessions.Load(sessionId)
	if !ok {
		mgr.sids.Delete(sid)
		logger.Log.Error("[LogicManager]GetSessionBySid sessionId not found=[%d]", sessionId)
		return nil
	}
	return session.(*logicSession)
}

func (mgr *LogicManager) addClient(client *logicSession) {
	mgr.mtx.Lock()
	defer mgr.mtx.Unlock()
	mgr.lgcSessions.Store(client.SessionID(), client)
}

func (mgr *LogicManager) deleteClient(client *logicSession) {
	mgr.mtx.Lock()
	defer mgr.mtx.Unlock()

	mgr.lgcSessions.Delete(client.sessionID)
	for _, sid := range client.sids {
		mgr.sids.Delete(sid)
	}
	logger.Log.Warn("[LogicManager]deleteClient delete session=[%v] sids=[%v]", client.SessionID(), client.sids)
}

func (mgr *LogicManager) bindClient(uniqueId uint64, aid []uint64) bool {
	mgr.mtx.Lock()
	defer mgr.mtx.Unlock()

	sess := mgr.GetSession(uniqueId)
	if sess == nil {
		logger.Log.Error("LogicManager[bindClient] uniqueId=[%d] not exist", uniqueId)
		return false
	}
	//服务器session
	mgr.lgcSessions.Store(uniqueId, sess)

	//服务器id
	for _, tid := range aid {
		if _, ok := mgr.sids.Load(tid); ok {
			logger.Log.Error("[LogicManager]bindClient tid=[%v] repeated", tid)
			return false
		}
	}

	//绑定到服务器id组
	for _, tid := range aid {
		mgr.sids.Store(tid, uniqueId)
	}

	//绑定到sess
	sess.sids = aid
	return true
}
