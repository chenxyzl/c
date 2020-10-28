package cobra

import (
	"github.com/topfreegames/pitaya/logger"
)

type Config struct {
	//ip:port
	Address string `xml:"address"`
	//read config
	MaxReadMsgSize   int `xml:"read_max_size"`
	ReadMsgQueueSize int `xml:"read_queue_size"`
	ReadTimeOut      int `xml:"read_timeout"`
	//write config
	MaxWriteMsgSize   int `xml:"write_max_size"`
	WriteMsgQueueSize int `xml:"write_queue_size"`
	WriteTimeOut      int `xml:"write_timeout"`
}

type NetConfig struct {
	Config
	WsAddress  string `xml:"ws_address"`
	Handler    string `xml:"handler"`
	WebCert    string `xml:"webcert"`
	WebCertKey string `xml:"webcert_key"`
	Protocol   string `xml:"protocl"`
}

func (this *Config) Check() bool {
	if this.MaxWriteMsgSize == 0 {
		logger.Log.Error("[Config] MaxWriteMsgSize error")
		return false
	}
	if this.WriteMsgQueueSize == 0 {
		logger.Log.Error("[Config] WriteMsgQueueSize error")
		return false
	}
	if this.MaxReadMsgSize == 0 {
		logger.Log.Error("[Config] MaxReadMsgSize error")
		return false
	}
	return true
}

func (this *NetConfig) Check() bool {
	return this.Config.Check()
}
