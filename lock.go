package redislock

import (
	"context"
	"errors"
	"fmt"
	"redis_lock/utils"
	"sync/atomic"
	"time"

	"github.com/gomodule/redigo/redis"
)

// 用于与 key 拼接，形成真正插入 redis 的 key
const RedisLockKeyPrefix = "REDIS_LOCK_PREFIX_"

var ErrLockAcquiredByOthers = errors.New("lock is acquired by others")

// 发生 redis.ErrNil 错误时，要进行重试
var ErrNil = redis.ErrNil

func IsRetryableErr(err error) bool {
	return errors.Is(err, ErrLockAcquiredByOthers)
}

// 基于 redis 实现的分布式锁，保证不可重入性、对称性
type RedisLock struct {
	LockOptions
	key    string
	token  string // 当前加锁协程唯一标识
	client LockClient

	runningDog int32              // 看门狗运行标识
	stopDog    context.CancelFunc // 停止看门狗(的 context，关闭 Context.Done() channel)

	logger *logx
}

// NewXxx 不带方法接收者，其作为工厂函数，创建对象；而不是作为对象自身的方法
func NewRedisLock(key string, client LockClient, opts ...LockOption) *RedisLock {
	r := RedisLock{
		key:    key,
		token:  utils.GetProcessAndGoroutineIDStr(),
		client: client,
		logger: newLogger(),
	}

	for _, opt := range opts {
		// opt 是一系列工厂方法 WithMaxIdle 等，返回的闭包
		opt(&r.LockOptions)
	}

	repairLock(&r.LockOptions)
	return &r
}

// 加锁
func (r *RedisLock) Lock(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			return
		}
		// TODO: 加锁成功，启动 watch dog
		// 加锁成功的情况下，会启动看门狗
		// 关于该锁本身是不可重入的，所以不会出现同一把锁下看门狗重复启动的情况
		r.watchDog(ctx)
	}()

	// 尝试获取锁
	err = r.tryLock(ctx)
	if err == nil {
		return nil
	}

	// 非阻塞模式，直接返回错误
	if !r.isBlock {
		return err
	}

	// 判断错误是否可以允许重试，不可允许的类型则直接返回错误、
	if !IsRetryableErr(err) {
		return err
	}

	// 阻塞模式，轮询获取锁
	err = r.blockingLock(ctx)
	return
}

// 启动看门狗
func (r *RedisLock) watchDog(ctx context.Context) {
	// 非看门狗模式，直接返回
	if !r.watchDogMode {
		return
	}

	// 确保在运行的看门狗的唯一性
	// 一把锁只能由一个看门狗，去为其续约
	for !atomic.CompareAndSwapInt32(&r.runningDog, 0, 1) {
	}

	// TODO *** 创建一个子 ctx，启动看门狗
	ctx, r.stopDog = context.WithCancel(ctx)
	// ctx, r.stopDog = context.WithTimeout(ctx, 30*time.Second)
	go func() {
		defer func() {
			atomic.StoreInt32(&r.runningDog, 0)
		}()
		r.runWatchDog(ctx)
	}()
}

func (r *RedisLock) runWatchDog(ctx context.Context) {
	ticker := time.NewTicker(WatchDogWorkStepSeconds * time.Second)
	defer ticker.Stop()
	r.logger.Info("test1")
	for range ticker.C {
		r.logger.Info("test2")
		select {
		case <-ctx.Done():
			return
		default:
		}
		// 每 WatchDogWorkStepSeconds 秒续约一次，每次续约 WatchDogWorkStepSeconds 秒(加 3 秒为了避免网络延迟，导致续约失败)
		_ = r.DelayExpire(ctx, WatchDogWorkStepSeconds+3)
	}
}

// 锁的续约，基于 lua 脚本
func (r *RedisLock) DelayExpire(ctx context.Context, expireSeconds int64) error {
	// TODO 不要写成 r.key！！！ 身份校验无法通过！
	keyAndArgs := []interface{}{r.getLockKey(), r.token, expireSeconds}
	reply, err := r.client.Eval(ctx, LuaCheckAndExpireDistributionLock, 1, keyAndArgs)

	r.logger.Debug("续约触发", keyAndArgs, reply, err)
	if err != nil {
		r.logger.Error("续约失败", keyAndArgs, reply, err)
		return err
	}
	if ret, _ := reply.(int64); ret != 1 {
		r.logger.Error("续约失败2", keyAndArgs, reply, err)
		return errors.New("can not expire lock without ownership of lock")
	}
	r.logger.Info("续约成功")
	return nil
}

// 尝试获取锁 (执行 SetNX，查看是否成功)
func (r *RedisLock) tryLock(ctx context.Context) (err error) {
	reply, err := r.client.SetNX(ctx, r.getLockKey(), r.token, r.expireSeconds)
	r.logger.Debug("tryLock: SETNX result, key=%s, reply=%v, err=%v", r.getLockKey(), reply, err)

	// TODO 关键！！ 发生 redis 返回为空错误时，不能直接返回错误，要将其作为 ErrLockAcquiredByOthers 错误返回
	if errors.Is(err, redis.ErrNil) {
		r.logger.Debug("redis.ErrNil 代表执行 redis SETNX 失败，未获取到锁，可重试: reply: %d, err: %v", reply, ErrLockAcquiredByOthers)
		return fmt.Errorf("redis.ErrNil 代表执行 redis SETNX 失败，未获取到锁，可重试: reply: %d, err: %w", reply, ErrLockAcquiredByOthers)
	}

	if err != nil {
		return err
	}

	return nil
}

func (r *RedisLock) getLockKey() string {
	return RedisLockKeyPrefix + r.key
}

// 阻塞模式，持续轮询去获取锁
func (r *RedisLock) blockingLock(ctx context.Context) error {
	// 阻塞模式等锁时间上限
	timeoutCh := time.After(time.Duration(r.blockWaitingSeconds) * time.Second)
	// 轮询 ticker，每隔 50 ms 尝试取锁一次
	ticker := time.NewTicker(time.Duration(50) * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		select {
		// ctx 终止了
		case <-ctx.Done():
			return fmt.Errorf("lock failed, ctx timeout, err: %w", ctx.Err())
			// 阻塞等锁达到上限时间
		case <-timeoutCh:
			return fmt.Errorf("block waiting time out, err: %w", ErrLockAcquiredByOthers)
		// 放行
		default:
		}

		// 尝试取锁
		err := r.tryLock(ctx)
		if err == nil {
			// 加锁成功，返回结果
			return nil
		}

		// 不可重试类型的错误，直接返回
		if !IsRetryableErr(err) {
			return err
		}
	}

	// 不可达
	return nil
}

// 解锁，基于 lua 脚本，实现身份验证与解锁的原子化操作
func (r *RedisLock) Unlock(ctx context.Context) error {
	defer func() {
		// TODO: 停止 watch dog
		r.logger.Info("解锁，看门狗关闭")
		r.stopDog()
	}()

	keyAndArgs := []interface{}{r.getLockKey(), r.token}
	reply, err := r.client.Eval(ctx, LuaCheckAndDeleteDistributionLock, 1, keyAndArgs)
	if err != nil {
		return err
	}

	// 判断解锁是否成功(执行 DEL 操作成功，返回 1)
	if ret, _ := reply.(int64); ret != 1 {
		return errors.New("can not unlock without ownership of lock")
	}

	return nil
}
