package util

import (
	"errors"
	"sync"
	"time"
)

// Snowflake 雪花算法生成器
type Snowflake struct {
	mu        sync.Mutex
	timestamp int64
	workerID  int64
	dataID    int64
	sequence  int64

	epoch     int64 // 起始时间戳
	wkBits    uint8 // 工作节点ID位数
	dataBits  uint8 // 数据中心ID位数
	seqBits   uint8 // 序列号位数
	maxWorker int64 // 最大工作节点ID
	maxData   int64 // 最大数据中心ID
	maxSeq    int64 // 最大序列号
	wkShift   uint8 // 工作节点ID左移位数
	dataShift uint8 // 数据中心ID左移位数
	timeShift uint8 // 时间戳左移位数
}

// Option 配置选项
type Option func(*Snowflake) error

// WithEpoch 设置起始时间戳（毫秒）
func WithEpoch(epoch int64) Option {
	return func(s *Snowflake) error {
		if epoch <= 0 {
			return errors.New("epoch must be positive")
		}
		s.epoch = epoch
		return nil
	}
}

// WithWorkerBits 设置工作节点ID位数
func WithWorkerBits(bits uint8) Option {
	return func(s *Snowflake) error {
		if bits <= 0 || bits > 20 {
			return errors.New("worker bits must be between 1 and 20")
		}
		s.wkBits = bits
		return nil
	}
}

// WithDataBits 设置数据中心ID位数
func WithDataBits(bits uint8) Option {
	return func(s *Snowflake) error {
		if bits <= 0 || bits > 20 {
			return errors.New("data bits must be between 1 and 20")
		}
		s.dataBits = bits
		return nil
	}
}

// WithSequenceBits 设置序列号位数
func WithSequenceBits(bits uint8) Option {
	return func(s *Snowflake) error {
		if bits <= 0 || bits > 20 {
			return errors.New("sequence bits must be between 1 and 20")
		}
		s.seqBits = bits
		return nil
	}
}

// NewSnowflake 创建雪花算法生成器
// workerID: 工作节点ID
// dataID: 数据中心ID
// options: 配置选项
func NewSnowflake(workerID, dataID int64, options ...Option) (*Snowflake, error) {
	s := &Snowflake{
		epoch:    1609459200000, // 默认起始时间：2021-01-01 00:00:00
		wkBits:   5,             // 5位工作节点ID，支持32个节点
		dataBits: 5,             // 5位数据中心ID，支持32个数据中心
		seqBits:  12,            // 12位序列号，支持每个毫秒4096个ID
	}

	// 应用配置选项
	for _, opt := range options {
		if err := opt(s); err != nil {
			return nil, err
		}
	}

	// 计算最大值
	s.maxWorker = (1 << s.wkBits) - 1
	s.maxData = (1 << s.dataBits) - 1
	s.maxSeq = (1 << s.seqBits) - 1

	// 验证workerID和dataID
	if workerID < 0 || workerID > s.maxWorker {
		return nil, errors.New("invalid worker ID")
	}
	if dataID < 0 || dataID > s.maxData {
		return nil, errors.New("invalid data ID")
	}

	// 计算移位位数
	s.wkShift = s.seqBits
	s.dataShift = s.seqBits + s.wkBits
	s.timeShift = s.seqBits + s.wkBits + s.dataBits

	s.workerID = workerID
	s.dataID = dataID
	s.sequence = 0
	s.timestamp = 0

	return s, nil
}

// NextID 生成下一个唯一ID
func (s *Snowflake) NextID() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()

	// 如果当前时间小于上次生成ID的时间，说明发生了时钟回拨
	if now < s.timestamp {
		return 0, errors.New("clock moved backwards")
	}

	// 如果是同一毫秒，序列号加1
	if now == s.timestamp {
		s.sequence++
		// 如果序列号超过最大值，等待下一毫秒
		if s.sequence > s.maxSeq {
			for now <= s.timestamp {
				now = time.Now().UnixMilli()
			}
			s.sequence = 0
		}
	} else {
		// 不同毫秒，序列号重置为0
		s.sequence = 0
	}

	s.timestamp = now

	// 组合ID：时间戳 << 时间移位 + 数据中心ID << 数据移位 + 工作节点ID << 工作移位 + 序列号
	id := (now-s.epoch)<<s.timeShift |
		s.dataID<<s.dataShift |
		s.workerID<<s.wkShift |
		s.sequence

	return id, nil
}

// ParseID 解析ID，返回各个组成部分
func (s *Snowflake) ParseID(id int64) map[string]int64 {
	timestamp := (id >> s.timeShift) + s.epoch
	dataID := (id >> s.dataShift) & s.maxData
	workerID := (id >> s.wkShift) & s.maxWorker
	sequence := id & s.maxSeq

	return map[string]int64{
		"timestamp": timestamp,
		"dataID":    dataID,
		"workerID":  workerID,
		"sequence":  sequence,
	}
}
