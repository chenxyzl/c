package gate

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path"
	"strconv"
	"syscall"
	"time"
	common_constant "c/common/constant"
	"c/framework/discovery"
	"c/framework/mylog4go"
	"c/framework/util"
	"c/gate/configuration"
	"c/gate/global"
	"c/gate/sess"
	"c/protos/in/web_api"
	"c/table"
	"c/version"

	"github.com/topfreegames/pitaya/logger"
)

var (
	svrConfig = flag.String("config", "../config/gate.xml", "service config path")
	logConfig = flag.String("logcfg", "../config/gate_log.xml", "service log config path")
	debugLog  = flag.Bool("log", false, "show console log")
	id        = flag.Uint64("id", 0, "id")
	qps       = flag.Uint64("qps", 10, "qps")
)

var (
	logicMgr  *sess.LogicManager
	clientMgr *sess.ClientManager
)

//Run the comet application
func Run() {
	logger.Log.Warn("[gate]Run begin...")
	// 初始化log
	logger.Log.Global.LoadConfiguration(*logConfig, nil)
	if *debugLog {
		logger.Log.AddFilter("colorLogger", logger.Log.DEBUG, mylog4go.NewColorConsoleLogWriter())
	}

	if err := configuration.ParseConfig(*svrConfig); err != nil {
		panic(fmt.Sprintf("load config %v fail: %v", *svrConfig, err))
	}

	//global
	global.InitGlobal(*id)

	//监听logic
	logicMgr = sess.NewLogicManager(configuration.GetInBoundCfg(), configuration.GetMsgQueueSize())
	logicMgr.GoRun()
	defer logicMgr.Close()

	//监听client
	clientMgr = sess.NewClientManager(configuration.GetOutBoundCfg(), configuration.GetMsgQueueSize(), *qps)
	clientMgr.GoRun()
	defer clientMgr.Close()

	//注册gate状态
	s := registerState()
	if s != nil {
		defer s.Stop()
	}

	//注册自己到etcd的gateway
	s1 := registerGateway()
	if s1 != nil {
		defer s1.Stop()
	}

	// ticker
	aliveTicker := time.NewTicker(time.Minute * 5)
	defer aliveTicker.Stop()

	ticker := time.NewTicker(time.Millisecond * 200)
	defer ticker.Stop()

	// system signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	//主循环
QUIT:
	for {
		select {
		case sig := <-sigs:
			logger.Log.Warn("[main] sig = %d", sig)
			if sig == syscall.SIGHUP {
				table.Reload()
				logger.Log.Warn("[main] table.Reload()r")
				continue
			} else if sig == syscall.SIGQUIT {
				logger.Log.Error("[main] cross is stopped by force!")
			}
			break QUIT
		case <-logicMgr.WatchChan():
			logger.Log.Info("[main] gate etcd watch close...")
			break QUIT
		}
	}
}

func registerState() *discovery.Service {
	serverStateCfg := configuration.GetGateStateCfg()
	// register server state to etcd
	if len(serverStateCfg.Servers) > 0 {
		id := uint64(time.Now().UnixNano())
		pwd, _ := os.Getwd()
		serverStateCfg.Target = path.Join(serverStateCfg.Target, common_constant.ServerName_Gateway, strconv.FormatUint(id, 10))
		state := &web_api.ServerState{
			ServerId:   id,
			Ip:         util.GetOutboundIP(),
			LaunchTime: time.Now().Unix(),
			Version:    version.BuildVersion,
			CommitId:   version.CommitID,
			Dir:        pwd,
		}
		stateStr, _ := json.Marshal(state)
		stateDs := discovery.NewService(&serverStateCfg, string(stateStr))
		go func() {
			if err := stateDs.Run(); nil != err {
				logger.Log.Error("[main] etcd server state error:[%s]", err.Error())
			}
		}()
		logger.Log.Info("[main] registerSelf [%s] into etcd", serverStateCfg.Target)
		return stateDs
	}
	return nil
}

func registerGateway() *discovery.Service {
	cfg := configuration.GetGateAliveCfg()
	// register server state to etcd
	if len(cfg.Servers) > 0 {
		cfg.Target = fmt.Sprintf("%s%s%s", cfg.Target, util.GetOutboundIP(), configuration.GetOutBoundCfg().Address)
		stateDs := discovery.NewService(&cfg, fmt.Sprintf("%d", 0))
		go func() {
			if err := stateDs.Run(); nil != err {
				logger.Log.Error("[main] etcd server state error:[%s]", err.Error())
			}
		}()
		logger.Log.Info("[main] registerSelf [%s] into etcd", cfg.Target)
		return stateDs
	}
	return nil
}
