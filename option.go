package redislock

import "time"

const (
	// 默认连接池超过 10 s 释放连接
	DefaultIdleTimeoutSeconds = 10
	// 默认最大激活连接数
	DefaultMaxActive = 100
	// 默认最大空闲连接数
	DefaultMaxIdle = 20

	// 默认的分布式锁过期时间
	DefaultLockExpireSeconds = 10
	// 看门狗工作时间间隙
	WatchDogWorkStepSeconds = 3
)

// 连接池客户端参数
type ClientOptions struct {
	maxIdle            int
	idleTimeoutSeconds int
	maxActive          int
	wait               bool
	// 必填参数
	network  string
	address  string
	password string
}

/*
	函数式选项模式：定义一个选项函数类型 和 一系列选项创建函数
	清晰和可读性强： 调用代码时，参数名称一目了然 (WithMaxIdle(20))，比直接传入数字更清晰。
	灵活性： 你可以只传入你关心的选项，未传入的选项会使用默认值。参数顺序不再重要。
	可扩展性： 添加新的配置选项只需要添加一个新的 WithXxx 函数，而无需修改 ClientOptions 结构体或构造函数的签名。
	这符合开放-封闭原则（Open-Closed Principle）。
	避免参数爆炸： 构造函数不再需要接受大量参数。
*/

// 选项函数类型
type ClientOption func(c *ClientOptions)

// 一组选项创建函数，将传入的参数，形成闭包返回
// 返回的匿名函数, 后续被自动执行时，把闭包内的值赋给 ClientOptions
func WithMaxIdle(maxIdle int) ClientOption {
	return func(c *ClientOptions) {
		c.maxIdle = maxIdle
	}
}

func WithIdleTimeoutSeconds(idleTimeoutSeconds int) ClientOption {
	return func(c *ClientOptions) {
		c.idleTimeoutSeconds = idleTimeoutSeconds
	}
}

func WithMaxActive(maxActive int) ClientOption {
	return func(c *ClientOptions) {
		c.maxActive = maxActive
	}
}

func WithWaitMode() ClientOption {
	return func(c *ClientOptions) {
		c.wait = true
	}
}

// 确保参数合法
func repairClient(c *ClientOptions) {
	if c.maxIdle < 0 {
		c.maxIdle = DefaultMaxIdle
	}

	if c.idleTimeoutSeconds < 0 {
		c.idleTimeoutSeconds = DefaultIdleTimeoutSeconds
	}

	if c.maxActive < 0 {
		c.maxActive = DefaultMaxActive
	}
}

// 分布式锁参数
type LockOptions struct {
	isBlock             bool
	blockWaitingSeconds int64
	expireSeconds       int64
	watchDogMode        bool // 不显式指定锁的过期时间，会自动启动看门狗 (自动更新过期时间)
}

type LockOption func(*LockOptions)

func WithBlock() LockOption {
	return func(lo *LockOptions) {
		lo.isBlock = true
	}
}

func WithBlockWaitingSeconds(waitingSeconds int64) LockOption {
	return func(lo *LockOptions) {
		lo.blockWaitingSeconds = waitingSeconds
	}
}

func WithExpireSeconds(expireSeconds int64) LockOption {
	return func(lo *LockOptions) {
		lo.expireSeconds = expireSeconds
	}
}

func repairLock(lo *LockOptions) {
	if lo.isBlock && lo.blockWaitingSeconds <= 0 {
		// 默认阻塞等待时间上限为 5 秒
		lo.blockWaitingSeconds = 5
	}

	// ***倘若未设置分布式锁的过期时间，则会启动 watchdog***
	if lo.expireSeconds > 0 {
		return
	}

	// 用户未显式指定锁的过期时间，此时会设定默认过期时间，并启动看门狗 (自动更新过期时间)
	lo.expireSeconds = DefaultLockExpireSeconds
	lo.watchDogMode = true
}

type RedLockOption func(*RedLockOptions)

type RedLockOptions struct {
	singleNodesTimeout time.Duration // 单节点获取锁过期时间，所有节点之和 小于 分布式锁过期时间的十分之一
	expireDuration     time.Duration // 分布式锁过期时间
}

func WithSingleNodesTimeout(singleNodesTimeout time.Duration) RedLockOption {
	return func(o *RedLockOptions) {
		o.singleNodesTimeout = singleNodesTimeout
	}
}

func WithRedLockExpireDuration(duration time.Duration) RedLockOption {
	return func(o *RedLockOptions) {
		o.expireDuration = duration
	}
}

// 每一个 redis 节点
type SingleNodeConf struct {
	Network  string
	Address  string
	Password string
	Opts     []ClientOption
}

func repairRedLock(o *RedLockOptions) {
	if o.singleNodesTimeout <= 0 {
		o.singleNodesTimeout = DefaultSingleLockTimeout
	}
}
