## RLock
由Golang实现的基于Redis的分布式锁。
### 引用 sdk
```
go get github.com/Hansisunstoppable/redis_lock
```
### 实现功能
- 互斥性：Redis 分布式锁可以保证同一时刻只有一个客户端可以获得锁，实现线程之间的互斥。
- 安全性：Redis 分布式锁采用原子操作，可以保证并发情况下锁的安全性，避免数据竞争、死锁等问题。
- 锁超时：为了避免某个客户端获取锁后失败而导致死锁，Redis 分布式锁可以设置锁超时时间，超过超时时间会自动释放锁。
- 可重入性：Redis 分布式锁可以支持同一个客户端多次获取同一个锁，避免嵌套调用时出现死锁。
- 高性能：Redis 是一个内存数据库，具有很高的读写性能，可以实现快速的加锁和解锁操作。
- 原子性：Redis 分布式锁的加锁和解锁操作使用原子命令，可以保证操作的原子性，避免并发下的竞争问题。
- 红锁：在红锁 RedLock 实现中，会基于多数派准则进行 CAP 中一致性 C 和可用性 A 之间矛盾的缓和，保证在 RedLock 下所有 Redis 节点中达到半数以上节点可用时，整个红锁就能够正常提供服务。
### 快速开始
#### 非阻塞模式取锁
```go
package redis_lock

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func Test_blockingLock(t *testing.T) {
	// 请输入 redis 节点的地址和密码
	addr := "xxxx:xx"
	passwd := ""

	client := NewClient("tcp", addr, passwd)
	lock1 := NewRedisLock("test_key", client, WithExpireSeconds(1))
	lock2 := NewRedisLock("test_key", client, WithBlock(), WithBlockWaitingSeconds(2))

	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := lock1.Lock(ctx); err != nil {
			t.Error(err)
			return
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := lock2.Lock(ctx); err != nil {
			t.Error(err)
			return
		}
	}()

	wg.Wait()

	t.Log("success")
}
```
#### 阻塞模式取锁
```go
package redis_lock

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func Test_nonblockingLock(t *testing.T) {
	// 请输入 redis 节点的地址和密码
	addr := "xxxx:xx"
	passwd := ""

	client := NewClient("tcp", addr, passwd)
	lock1 := NewRedisLock("test_key", client, WithExpireSeconds(1))
	lock2 := NewRedisLock("test_key", client)

	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := lock1.Lock(ctx); err != nil {
			t.Error(err)
			return
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := lock2.Lock(ctx); err == nil || !errors.Is(err, ErrLockAcquiredByOthers) {
			t.Errorf("got err: %v, expect: %v", err, ErrLockAcquiredByOthers)
			return
		}
	}()

	wg.Wait()
	t.Log("success")
}
```
#### 启用看门狗模式
```go
func repairLock(o *LockOptions) {
	if o.isBlock && o.blockWaitingSeconds <= 0 {
		// 默认阻塞等待时间上限为 5 秒
		o.blockWaitingSeconds = 5
	}

	// 倘若未设置分布式锁的过期时间，则会启动 watchdog
	if o.expireSeconds > 0 {
		return
	}

	// 用户未显式指定锁的过期时间，则此时会启动看门狗
	o.expireSeconds = DefaultLockExpireSeconds
	o.watchDogMode = true
}
```
#### 红锁
```go
package redis_lock

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func Test_redLock(t *testing.T) {
	// 请输入三个 redis 节点的地址和密码
	addr1 := "xxxx:xx"
	passwd1 := ""

	addr2 := "yyyy:yy"
	passwd2 := ""

	addr3 := "zzzz:zz"
	passwd3 := ""

	// 直连三个 redis 节点的三把锁
	confs := []*SingleNodeConf{
		{
			Network:  "tcp",
			Address:  addr1,
			Password: passwd1,
		},
		{
			Network:  "tcp",
			Address:  addr2,
			Password: passwd2,
		},
		{
			Network:  "tcp",
			Address:  addr3,
			Password: passwd3,
		},
	}

	redLock, err := NewRedLock("test_key", confs, WithRedLockExpireDuration(10*time.Second), WithSingleNodesTimeout(100*time.Millisecond))
	if err != nil {
		return
	}

	ctx := context.Background()
	if err = redLock.Lock(ctx); err != nil {
		t.Error(err)
		return
	}

	if err = redLock.Unlock(ctx); err != nil {
		t.Error(err)
		return
	}

	t.Log("success")
}
```
