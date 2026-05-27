package sharksnowflake

import (
	"sync"
	"time"
)

// 改进型Snowflake算法生成全局唯一ID
// 1. 使用2025-01-01作为纪元，延长使用寿命
// 2. 序列号扩展到19位，支持更高的并发生成ID
// 3. 不支持机器ID,只可用于单机
// 4. Id结构: 41位时间戳 + 19位序列号
// 5. 每毫秒支持生成520000个ID
/*
	本项目用法:
	1. 生成N个ID,存入Redis列表,Redis为分片集群,可以是多个不同key列表,支持海量ID存储和高并发读写
	2. 业务用时从Redis列表弹出ID使用
	3. 定时检查Redis列表ID数量,不足时补充ID
*/
const (
	epoch         = 1735660800 // 2025-01-01 00:00:00
	maxSequenceId = 520000     // 19 bits
)

type Snowflake struct {
	mu         sync.Mutex
	sequenceId int64 // 当前序列号
	timeStamp  int64 // 上次生成Id的时间戳
}

func NewSnowflake() *Snowflake {
	return &Snowflake{
		sequenceId: 0,
		timeStamp:  0,
	}
}

func (s *Snowflake) Generate() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	for {
		timestamp := time.Now().Unix()
		if timestamp < s.timeStamp {
			time.Sleep(time.Millisecond)
			continue
		}
		sleeping := false
		if s.timeStamp == timestamp {
			s.sequenceId++
			if s.sequenceId >= maxSequenceId {
				time.Sleep(time.Millisecond)
				sleeping = true
			}
		} else {
			s.sequenceId = 0
		}
		s.timeStamp = timestamp
		if !sleeping {
			break
		}
	}
	id := (s.timeStamp-epoch)<<19 | s.sequenceId
	return id
}
