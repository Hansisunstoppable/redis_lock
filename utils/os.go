package utils // utils，工具包，使用 进程 ID 和 Goroutine ID 的组合字符串，来生成唯一的 token

import (
	"fmt"     // 导入 fmt 包，用于格式化字符串（例如 Sprintf）
	"os"      // 导入 os 包，用于与操作系统交互（例如获取进程 ID）
	"runtime" // 导入 runtime 包，用于获取运行时信息（例如 Goroutine 堆栈）
	"strconv" // 导入 strconv 包，用于字符串和基本数据类型之间的转换（例如 int 转 string）
	"strings" // 导入 strings 包，用于字符串操作（例如分割、去除空格）
)

// GetCurrentProcessID 获取当前进程的 ID
// 返回值: string 类型，表示当前进程的 ID
func GetCurrentProcessID() string {
	// os.Getpid() 返回调用方进程的进程 ID (PID)
	// strconv.Itoa 将整数 PID 转换为字符串
	return strconv.Itoa(os.Getpid())
}

// GetCurrentGoroutineID 获取当前的协程ID
// 注意：获取 Goroutine ID 不是 Go 语言官方推荐的做法，因为 Goroutine ID 在 Go 运行时内部使用，
// 且不保证稳定性和唯一性（例如，Goroutine ID 可能会被重用）。
// 在生产环境中，更推荐使用 context.WithValue 来传递唯一的请求 ID 或 trace ID，
func GetCurrentGoroutineID() string {
	// 创建一个足够大的字节切片（缓冲区），用于存储 Goroutine 的堆栈信息。
	// 128 字节通常足够存储 Goroutine ID 部分。
	buf := make([]byte, 128)

	// runtime.Stack(buf, false) 将当前 Goroutine 的堆栈跟踪信息写入 buf。
	// 第二个参数 false 表示不包括所有 Goroutine 的堆栈，只包括当前 Goroutine。
	// 它返回实际写入 buf 的字节数。
	buf = buf[:runtime.Stack(buf, false)]

	// 将字节切片转换为字符串，以便进行字符串操作。
	stackInfo := string(buf)

	// 从堆栈信息字符串中提取 Goroutine ID。
	// 堆栈信息通常以 "goroutine 12345 [running]:" 这样的格式开始。
	// 1. strings.Split(stackInfo, "[running]")[0]:
	//    - 以 "[running]" 为分隔符将字符串分割。
	//    - 取第一个部分，即 "goroutine 12345 "（注意后面有个空格）。
	// 2. strings.Split(..., "goroutine")[1]:
	//    - 以 "goroutine" 为分隔符将上一步的结果分割。
	//    - 取第二个部分，即 " 12345 "（注意前后都有空格）。
	// 3. strings.TrimSpace(...):
	//    - 去除字符串两端的空格，得到纯粹的 Goroutine ID，例如 "12345"。
	return strings.TrimSpace(strings.Split(strings.Split(stackInfo, "[running]")[0], "goroutine")[1])
}

// GetProcessAndGoroutineIDStr 获取当前进程 ID 和 Goroutine ID 的组合字符串
func GetProcessAndGoroutineIDStr() string {
	// fmt.Sprintf 用于格式化字符串，将获取到的进程 ID 和协程 ID 组合起来
	// 中间用下划线 "_" 连接
	return fmt.Sprintf("%s_%s", GetCurrentProcessID(), GetCurrentGoroutineID())
}
