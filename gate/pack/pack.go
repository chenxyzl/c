package pack // Package cluster_network 是可以自动注册发现tcp网络的集群
import (
	"c/gate/cobra"
)

// Message 消息体
type Message struct {
	SessionID uint64
	PH        *cobra.PackHead
	Data      []byte
}
