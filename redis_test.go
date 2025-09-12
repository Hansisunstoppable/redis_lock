package redislock

import (
	"context"
	"runtime/debug"
	"sync"
	"testing"
	"time"
)

// go test -count=1 -run ^Test_NewClient$ redis_lock
func Test_NewClient(t *testing.T) {
	addr := "172.17.224.1:6379"
	passwd := ""

	client := NewClient("tcp", addr, passwd)
	ctx := context.Background()
	a, err := client.Get(ctx, "suck")
	if err != nil {
		t.Error(err)
	}
	t.Log(a)
}

func Test_blockingLock(t *testing.T) {
	addr := "172.17.224.1:6379"
	passwd := ""

	client := NewClient("tcp", addr, passwd)
	ctx := context.Background()
	lock1 := NewRedisLock("test_ke", client, WithExpireSeconds(1), WithBlock(), WithBlockWaitingSeconds(10))
	lock2 := NewRedisLock("test_ke", client, WithExpireSeconds(1), WithBlock(), WithBlockWaitingSeconds(10))

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := lock1.Lock(ctx); err != nil {
			t.Errorf("lock1.Lock failed: %v\nStack trace: %s", err, debug.Stack())
			return
		} else {
			t.Log("lock1.Lock success")
		}
	}()
	// time.Sleep(2 * time.Second)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := lock2.Lock(ctx); err != nil {
			t.Errorf("lock2.Lock failed: %v\nStack trace: %s", err, debug.Stack())
			return
		} else {
			t.Log("lock2.Lock success")
		}
	}()

	wg.Wait()

	t.Log("success")
}

func Test_WatchDog(t *testing.T) {
	addr := "172.17.224.1:6379"
	passwd := ""

	client := NewClient("tcp", addr, passwd)

	// 30 s 超时的 ctx，确保 watchdog 续约触发
	parentCtx, parentCancel := context.WithTimeout(context.Background(), 30*time.Second)
	// parentCtx := context.Background()
	defer parentCancel() // 确保 Context 最终被取消
	lock1 := NewRedisLock("testc", client, WithBlock(), WithBlockWaitingSeconds(10))

	if err := lock1.Lock(parentCtx); err != nil {
		t.Errorf("lock1.Lock failed: %v\nStack trace: %s", err, debug.Stack())
		return
	} else {
		t.Log("lock1.Lock success")
	}
	defer lock1.Unlock(parentCtx)

	// 模拟业务逻辑执行时间
	// 这个时间应该长于锁的过期时间 (10秒)，且足以触发多次看门狗续约 (看门狗每 3 秒续约)
	businessExecutionDuration := 18 * time.Second

	t.Logf("模拟业务逻辑执行 %s，观察看门狗续约情况...", businessExecutionDuration)
	time.Sleep(businessExecutionDuration) // 让测试函数在这里暂停

	t.Log("业务逻辑执行完毕。")

	// 稍微等待，确保看门狗协程有时间接收到取消信号并退出
	time.Sleep(2 * time.Second)
}

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
