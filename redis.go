package redislock

import (
	"context"
	"strings"
	"time"

	"github.com/gomodule/redigo/redis"
)

type LockClient interface {
	SetNX(ctx context.Context, key, value string, expireSeconds int64) (int64, error)
	Eval(ctx context.Context, src string, keyCount int, keyAndArgs []interface{}) (interface{}, error)
}

type Client struct {
	ClientOptions
	pool *redis.Pool
}

// opts 为选项函数类型，是选项创建函数(WithMaxIdle 等) 返回的闭包
func NewClient(network, address, password string, opts ...ClientOption) *Client {
	c := Client{
		ClientOptions: ClientOptions{
			network:  network,
			address:  address,
			password: password,
		},
	}

	for _, opt := range opts {
		// 向配置方法传入 *ClientOptions, 自动执行闭包，完成配置
		opt(&c.ClientOptions)
	}

	repairClient(&c.ClientOptions)

	// 获取 Redis 连接池
	pool := c.getRedisPool()

	// 返回一个新的 Client 对象指针，而不是 c.pool=pool, return &c，为什么？
	// 将 ClientOptions 和 pool 解耦，ClientOptions 只是为了生成 pool 所需要的配置
	// Client 对象实际上只关注 pool, 返回只有 pool 的 Client，ClientOptions 的生命周期就结束了！
	// 也避免了后续外部可以直接访问到 ClientOptions 的参数
	return &Client{
		pool: pool,
	}
}

// 根据 Client 的各种参数(通过闭包设置参数)，来创建 Redis 连接池
func (c *Client) getRedisPool() *redis.Pool {
	return &redis.Pool{
		MaxIdle:     c.maxIdle,
		IdleTimeout: time.Duration(c.idleTimeoutSeconds) * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := c.getRedisConn()
			if err != nil {
				return nil, err
			}
			return c, nil
		},
		MaxActive: c.maxActive,
		Wait:      c.wait,
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}

// 从 Redis 连接池获取可以连接，该连接支持 Context 的取消和超时
func (c *Client) getConn(ctx context.Context) (redis.Conn, error) {
	return c.pool.GetContext(ctx)
}

// Redis 拨号连接（dial: 拨号，用 address等 option）
func (c *Client) getRedisConn() (redis.Conn, error) {
	if c.address == "" {
		panic("redis address is empty")
	}

	var dialOption []redis.DialOption
	if len(c.password) > 0 {
		dialOption = append(dialOption, redis.DialPassword(c.password))
	}
	conn, err := redis.DialContext(context.Background(),
		c.network, c.address, dialOption...)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// 一组操作 redis 的方法（redis 连接支持 ctx 取消与超时）
// Get, Set, SetNX, Del, Incr
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	if key == "" {
		// return "", errors.New("redis GET key can't be empty")
		panic("redis GET key can't be empty")
	}
	conn, err := c.pool.GetContext(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	return redis.String(conn.Do("GET", key))
}

func (c *Client) Set(ctx context.Context, key, value string, expireSeconds int64) (int64, error) {
	if key == "" || value == "" {
		panic("redis SET key or value can't be empty")
	}
	conn, err := c.pool.GetContext(ctx)
	if err != nil {
		return -1, err
	}
	defer conn.Close()

	resp, err := conn.Do("SET", key, value)
	if err != nil {
		return -1, err
	}

	if respStr, ok := resp.(string); ok && strings.ToLower(respStr) == "ok" {
		return 1, nil
	}

	// 将 redis 命令的返回结果，转换为 go 的 int64
	return redis.Int64(resp, err)
}

func (c *Client) SetNX(ctx context.Context, key, value string, expireSeconds int64) (int64, error) {
	if key == "" || value == "" {
		panic("redis SET key or value can't be empty")
	}

	// 等于 conn, err := c.pool.GetContext(ctx) 吗？
	conn, err := c.getConn(ctx)
	if err != nil {
		return -1, err
	}
	defer conn.Close()

	// EX: 设置过期时间
	// NX: not exist, key 不存在，才会创建成功；失败是返回 nil
	reply, err := conn.Do("SET", key, value, "EX", expireSeconds, "NX")
	if err != nil {
		return -1, err
	}

	if respStr, ok := reply.(string); ok && strings.ToLower(respStr) == "ok" {
		return 1, nil
	}
	return redis.Int64(reply, err)
}

func (c *Client) Del(ctx context.Context, key string) error {
	if key == "" {
		panic("redis SET key can't be empty")
	}

	conn, err := c.getConn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Do("DEL", key)
	return err
}

func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	if key == "" {
		panic("redis SET key can't be empty")
	}

	conn, err := c.getConn(ctx)
	if err != nil {
		return -1, err
	}
	defer conn.Close()

	return redis.Int64(conn.Do("INCR", key))
}

// Eval: redis 执行 lua 脚本的命令
// scr(ipt): lua 脚本源码
// keyCount: 接下来的参数值，key 的个数
// keyAndArgs：Lua 脚本中会用到的 key名 和 参数值
func (c *Client) Eval(ctx context.Context, src string, keyCount int, keyAndArgs []interface{}) (interface{}, error) {
	args := make([]interface{}, 2+len(keyAndArgs))
	args[0] = src
	args[1] = keyCount
	copy(args[2:], keyAndArgs)

	conn, err := c.getConn(ctx)
	if err != nil {
		return -1, err
	}
	defer conn.Close()

	// 不同的 Do 操作，会返回不同类型数据(GET:字符串、INCR:Int、LRANGE:列表 等)，因此需要定义空接口返回值
	return conn.Do("EVAL", args...)
}
