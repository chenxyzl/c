package util

import (
	"fmt"
	"sync"
	"time"
)

/*
+-----------------------------------------------------------+
| 42 Bit Timestamp | 10 Bit WorkID | 12 Bit Sequence ID |
+-----------------------------------------------------------+
*/
/*
const (
	nodeBits  uint8  = 10                    // 节点 ID 的位数
	stepBits  uint8  = 12                    // 序列号的位数
	nodeMax   uint64 = -1 ^ (-1 << nodeBits) // 节点 ID 的最大值，用于检测溢出
	stepMax   uint64 = -1 ^ (-1 << stepBits) // 序列号的最大值，用于检测溢出
	timeShift uint8  = nodeBits + stepBits   // 时间戳向左的偏移量
	nodeShift uint8  = stepBits              // 节点 ID 向左的偏移量
	Epoch     uint64 = 1288834974657         // timestamp 2006-03-21:20:50:14 GMT
)
*/

/*
+-----------------------------------------------------------+
| 42 Bit Timestamp | 14 Bit WorkID | 8 Bit Sequence ID |
+-----------------------------------------------------------+
*/

const (
	nodeBits  uint8  = 14                    // 节点 ID 的位数
	stepBits  uint8  = 8                     // 序列号的位数
	nodeMax   uint64 = -1 ^ (-1 << nodeBits) // 节点 ID 的最大值，用于检测溢出
	stepMax   uint64 = -1 ^ (-1 << stepBits) // 序列号的最大值，用于检测溢出
	timeShift uint8  = nodeBits + stepBits   // 时间戳向左的偏移量
	nodeShift uint8  = stepBits              // 节点 ID 向左的偏移量
	Epoch     uint64 = 1288834974657         // timestamp 2006-03-21:20:50:14 GMT
)

type UUID struct {
	mu        sync.Mutex // 添加互斥锁，保证并发安全
	timestamp int64      // 时间戳部分
	node      uint64     // 节点 ID 部分
	step      uint64     // 序列号 ID 部分
}

var defaultUUID *UUID

func SetNode(node uint64) error {
	uuid, err := newUUID(node)
	defaultUUID = uuid
	return err
}

func Generate() uint64 {
	if defaultUUID != nil {
		panic("defaultUUID not init. please call function SetNode")
	}
	return defaultUUID.generate()
}

//NewUUID 构造器
func newUUID(node uint64) (*UUID, error) {
	// 如果超出节点的最大范围，产生一个 error
	if node > nodeMax {
		//return nil, errors.New("Node number must be between 0 and 1023")
		return nil, fmt.Errorf("Node number must be between 0 and %d.", nodeMax)
	}
	// 生成并返回节点实例的指针
	return &UUID{
		timestamp: 0,
		node:      node,
		step:      0,
	}, nil
}

// Generate 生成唯一id
func (n *UUID) generate() uint64 {
	n.mu.Lock() // 保证并发安全, 加锁

	// 获取当前时间的时间戳 (毫秒数显示)
	now := time.Now().UnixNano() / 1e6

	if n.timestamp == now {
		n.step = (n.step + 1) & stepMax

		// 当前 step 用完
		if n.step == 0 {
			// 等待本毫秒结束
			for now <= n.timestamp {
				now = time.Now().UnixNano() / 1e6
			}
		}

	} else {
		// 本毫秒内 step 用完
		n.step = 0
	}

	n.timestamp = now
	// 移位运算，生产最终 ID
	result := (uint64(now)-Epoch)<<timeShift | (n.node << nodeShift) | (n.step)

	n.mu.Unlock() // 方法运行完毕后解锁

	return result
}
