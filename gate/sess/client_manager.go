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

// ClientManager session管理器
type ClientManager struct {
	sessionId    uint64
	mtx          sync.Mutex
	gm           *util.GroupManager
	cfg          *cobra.NetConfig
	msgQ         chan *pack.Message
	cltSessions  map[uint64]*ClientSession
	logicManager *LogicManager
	maxQps       uint64
}

var cltMgr *ClientManager

// NewClientManager 构造器
func NewClientManager(cfg *cobra.NetConfig, msgQueueSize int, maxQps uint64) *ClientManager {
	cltMgr = &ClientManager{
		cfg:         cfg,
		cltSessions: make(map[uint64]*ClientSession),
		gm:          util.NewGroupManager(),
		maxQps:      maxQps,
	}
	return cltMgr
}

// WatchChan
func (mgr *ClientManager) WatchChan() <-chan struct{} {
	return mgr.gm.Chan()
}

// Run
func (mgr *ClientManager) GoRun() {
	if len(mgr.cfg.Address) > 0 {
		//mgr.gm.Add(1)
		go func() {
			cobra.TCPServe(mgr, &mgr.cfg.Config)
			//mgr.gm.Done()
		}()
	}
	if len(mgr.cfg.WsAddress) > 0 {
		//mgr.gm.Add(1)
		go func() {
			cobra.WsServe(mgr, mgr.cfg)
			//mgr.gm.Done()
		}()
	}
}

// Close
func (mgr *ClientManager) Close() {
	//关闭watch
	mgr.gm.Close()
	mgr.gm.Wait()

	mgr.mtx.Lock()
	for _, client := range mgr.cltSessions {
		client.stop()
		client.wait()
		delete(mgr.cltSessions, client.SessionID())
	}
	mgr.mtx.Unlock()
	logger.Log.Info("[ClientManager] close...")
}

// NewSession 创建session回掉
func (mgr *ClientManager) NewSession() cobra.Sessioner {

	var client *ClientSession
	mgr.mtx.Lock()

	config := &actor.Config{
		Kind:      1,
		KindName:  "client_actor",
		MsgQSize:  mgr.cfg.MaxReadMsgSize,
		Rate:      1000, //Actor ticker 频率(单位millsecond)
		TimerSize: 8,
	}
	client = newClientSession(mgr, config, global.GenUUID())

	mgr.mtx.Unlock()
	if client != nil {
		go client.Run()
	}

	logger.Log.Info("[ClientManager]NewSession  session=[%d]", client.SessionID())

	return client
}

//GetQps qps
func (mgr *ClientManager) GetMaxQps() uint64 {
	return mgr.maxQps
}

// GetSession 获得Session
func (mgr *ClientManager) GetSession(sessionID uint64) *ClientSession {
	mgr.mtx.Lock()
	defer mgr.mtx.Unlock()
	client := mgr.cltSessions[sessionID]
	return client
}

func (mgr *ClientManager) addClient(client *ClientSession) {
	mgr.mtx.Lock()
	defer mgr.mtx.Unlock()
	mgr.cltSessions[client.SessionID()] = client
}

func (mgr *ClientManager) deleteClient(client *ClientSession) {
	mgr.mtx.Lock()
	defer mgr.mtx.Unlock()
	delete(mgr.cltSessions, client.SessionID())

}
