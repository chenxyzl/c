package configuration

import (
	"errors"
	"fmt"
	"c/framework/discovery"
	"c/framework/util"
	"c/gate/cobra"
)

//A Config provides a Comet Config.
type Config struct {
	StateEtcdConfig discovery.Config `xml:"state"`
	ReadQSize       int              `xml:"msg_queue_size"`
	InBoundZebra    cobra.Config     `xml:"inbound"`
	OutBound        cobra.NetConfig  `xml:"outbound"`
	OutEtcdConfig   discovery.Config `xml:"outbound_etcd"`
}

var config *Config

func ParseConfig(file string) error {
	config = &Config{}
	if err := util.LoadConfig(file, config); err != nil {
		return errors.New(fmt.Sprintf("ParseConfig config %v fail: %v", file, err))
	}
	return nil
}

//GetGateStateCfg creates a instance of CometConfig
func GetGateStateCfg() discovery.Config {
	return config.StateEtcdConfig
}

//GetGateAliveCfg creates a instance of CometConfig
func GetGateAliveCfg() discovery.Config {
	out := config.OutEtcdConfig
	return out
}

//GetInBoundCfg creates a instance of CometConfig
func GetInBoundCfg() *cobra.Config {
	return &config.InBoundZebra
}

//GetWebCfg creates a instance of CometConfig
func GetOutBoundCfg() *cobra.NetConfig {
	return &config.OutBound
}

//GetMsgQueueSize creates a instance of CometConfig
func GetMsgQueueSize() int {
	return config.ReadQSize
}
