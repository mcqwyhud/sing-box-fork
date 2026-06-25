//go:build !generate

package main

import "C"
import (
	"context"
	"sync"

	"github.com/sagernet/sing-box/log"
)

// ============================================================================
// 官方原版的 main 入口
// ============================================================================
func main() {
	if err := mainCommand.Execute(); err != nil {
		log.Fatal(err)
	}
}

// ============================================================================
// Unity DLL 导出的全局状态与上下文管理
// ============================================================================
var (
	ctxCancel context.CancelFunc
	mu        sync.Mutex
	isRunning bool
)

func startInternal(configPath string) int {
	ctx, cancel := context.WithCancel(context.Background())
	ctxCancel = cancel

	// 🔥 降维打击：直接模拟官方命令行传递参数的行为，调用原生的底层初始化
	osArgs := []string{"sing-box", "run", "-c", configPath}
	
	// 创建一个独立线程去跑原本的 Execute()，防止阻塞 Unity 的主线程
	go func() {
		defer func() { recover() }()
		mainCommand.SetArgs(osArgs[1:])
		if err := mainCommand.ExecuteContext(ctx); err != nil {
			log.Error(err)
		}
	}()

	isRunning = true
	return 0
}

func stopInternal() {
	if ctxCancel != nil {
		ctxCancel() // 触发 Context 取消，通知底层所有组件优雅退出
		ctxCancel = nil
	}
	isRunning = false
}

// ============================================================================
// CGO 导出函数 (Unity 直接调用的接口)
// ============================================================================

//export StartSingBox
func StartSingBox(configPath *C.char) int {
	mu.Lock()
	defer mu.Unlock()
	defer func() { recover() }()

	if isRunning {
		return 1
	}
	return startInternal(C.GoString(configPath))
}

//export ReloadSingBox
func ReloadSingBox(newConfigPath *C.char) int {
	mu.Lock()
	defer mu.Unlock()
	defer func() { recover() }()

	stopInternal()
	// 稍微等待原实例释放网络端口
	return startInternal(C.GoString(newConfigPath))
}

//export StopSingBox
func StopSingBox() {
	mu.Lock()
	defer mu.Unlock()
	defer func() { recover() }()

	stopInternal()
}
