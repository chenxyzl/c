package cobra

import (
	"fmt"
	"time"

	"github.com/topfreegames/pitaya/logger"
)

type Executer interface{}
type Messager interface{}

type Command interface {
	Execute(Executer, *PackHead, Messager) bool
}

type CommandM struct {
	cmdm           map[uint32]Command
	maxExecuteTime int64
	idBegin        uint32
	idEnd          uint32
	idPause        map[uint32]struct{}
}

func NewCommandM(begin, end uint32, max_execute_time int64) *CommandM {
	return &CommandM{
		cmdm:           make(map[uint32]Command),
		maxExecuteTime: max_execute_time,
		idBegin:        begin,
		idEnd:          end,
	}
}

func (this *CommandM) Register(id uint32, cmd Command) {
	if id > this.idBegin && id < this.idEnd {
		this.cmdm[id] = cmd
	} else {
		panic(fmt.Sprintf("register error id: %d", id))
	}
}

func (this *CommandM) Pause(id uint32) {
	this.idPause[id] = struct{}{}
}

func (this *CommandM) Recover(id uint32) {
	delete(this.idPause, id)
}

func (this *CommandM) Dispatcher(session Executer, ph *PackHead, data Messager) bool {
	if _, exist := this.idPause[ph.Cmd]; exist {
		logger.Log.Info("[CommandM] Dispatcher COMMAND PAUSE, uid: %d cmd: %d", ph.Uid, ph.Cmd)
		return false
	}
	if cmd, exist := this.cmdm[ph.Cmd]; exist {
		start := time.Now().UnixNano()
		ret := cmd.Execute(session, ph, data)
		execute_time := time.Now().UnixNano() - start
		if execute_time > this.maxExecuteTime {
			logger.Log.Info("[CommandM] Dispatcher COMMAND EXECUTE TIME, uid: %d cmd: %d execute_time: %d", ph.Uid, ph.Cmd, execute_time)
		}
		return ret
	}

	//logger.Log.Error("[Command] no find cmd, uid: %d cmd: %d", ph.Uid, ph.Cmd)

	return false
}
