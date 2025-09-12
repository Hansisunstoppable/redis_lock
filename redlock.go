package redislock

import (
	"context"
	"errors"
	"time"
)

// 单节点超时时间
const DefaultSingleLockTimeout = 50 * time.Millisecond

type RedLock struct {
	RedLockOptions

	locks []*RedisLock //  一组redis 锁结点

	logger
}

func NewRedLock(key string, confs []*SingleNodeConf, opts ...RedLockOption) (*RedLock, error) {
	// 三个以上节点，红锁才有意义
	if len(confs) < 3 {
		return nil, errors.New("can not use redLock less than 3 nodes")
	}
	r := RedLock{}
	for _, opt := range opts {
		opt(&r.RedLockOptions)
	}

	repairRedLock(&r.RedLockOptions)
	if r.expireDuration > 0 && time.Duration(len(confs))*r.singleNodesTimeout*10 > r.expireDuration {
		// 要求所有节点累计的时间 小于 分布式锁过期时间的十分之一
		return nil, errors.New("expire thresholds of single node is too long")
	}

	// 0: 初始长度（length）
	// len(confs): 容量（capacity）
	r.locks = make([]*RedisLock, 0, len(confs))
	// 根据传入的 confs，创建 n 个 redis 锁
	for _, conf := range confs {
		client := NewClient(conf.Network, conf.Address, conf.Password, conf.Opts...)
		r.locks = append(r.locks, NewRedisLock(key, client, WithExpireSeconds(int64(r.expireDuration.Seconds()))))
	}

	return &r, nil
}

// 加锁，用 successCnt 统计加锁成功的节点
func (r *RedLock) Lock(ctx context.Context) (err error) {
	var successCnt int
	for _, lock := range r.locks {
		startTime := time.Now()
		// 为每一个结点，创建一个带超时的 ctx
		_ctx, cancel := context.WithTimeout(ctx, r.singleNodesTimeout)
		defer cancel()
		err := lock.Lock(_ctx)
		cost := time.Since(startTime)
		if err == nil && cost <= r.singleNodesTimeout {
			successCnt++
		}
	}
	if successCnt < len(r.locks)/2+1 {
		// 加锁失败，广播解锁，释放资源
		r.Unlock(ctx)
		return errors.New("lock failed, 未取得多数席位")
	}
	return nil
}

// 解锁，所有节点广播解锁（遍历所有节点）
func (r *RedLock) Unlock(ctx context.Context) error {
	var err error
	for _, lock := range r.locks {
		if _err := lock.Unlock(ctx); _err != nil {
			// 出现一个错误非空，就去更新错误，并 继续遍历(要继续解锁，不能停止)
			err = _err
		}
	}
	return err
}
