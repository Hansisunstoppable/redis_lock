## RLock
A distributed lock base on Redis achieved by Golang.
- [简体中文](https://github.com/Hansisunstoppable/redis_lock/blob/main/README_ZH.md)
### Usage
```
go get github.com/Hansisunstoppable/redis_lock
```
### Achieve
- Mutual exclusion: Redis distributed lock can ensure that only one client can acquire the lock at the same time, realizing mutual exclusion between threads.
- Security: Redis distributed locks use atomic operations, which can ensure the security of locks under concurrent conditions and avoid problems such as data competition and deadlocks.
- Lock timeout: In order to avoid deadlock caused by a failure of a certain client after acquiring the lock, the Redis distributed lock can set the lock timeout period, and the lock will be released automatically when the timeout is exceeded.
- Reentrancy: Redis distributed locks can support the same client to acquire the same lock multiple times, avoiding deadlocks in nested calls.
- High performance: Redis is an in-memory database with high read and write performance, enabling fast locking and unlocking operations.
- Atomicity: The locking and unlocking operations of Redis distributed locks use atomic commands, which can ensure the atomicity of operations and avoid competition problems under concurrency.
- RedLock: In the implementation of RedLock, the contradiction between Consistency C and Availability A in CAP will be eased based on the majority principle, ensuring that when more than half of all Redis nodes under RedLock are available, the entire RedLock can be provided normally Serve.
### Quick Start
#### Non-blocking mode lock acquisition
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
#### Blocking mode lock acquisition
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
#### Activate WatchDog
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
#### RedLock
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
