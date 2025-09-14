## RLock
A distributed lock base on Redis achieved by Golang.
### Usage
```
go get github.com/Hansisunstoppable/redis_lock
```
### Achieve
- Supports lock acquisition via blocking polling and non-blocking modes
- Supports automatic extension of lock expiration time using watchdog mode
- Supports RedLock to address Redis's weak data consistency issues
### Quick Start
#### Non-blocking mode lock acquisition
```
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
```
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
```
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
```
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
