//go:build !generate

package main

import "C"
import (
	"context"
	"os"
	"sync"

	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing-box/log"
)

// 保留原生入口通过编译
func main() {
	if err := mainCommand.Execute(); err != nil {
		log.Fatal(err)
	}
}

// ============================================================================
// Unity DLL 全局状态管理
// ============================================================================
var (
	ctxCancel context.CancelFunc
	mu        sync.Mutex
	isRunning bool
)

func startInternal(configPath string) int {
	// 1. 拦截所有管道输出，阻止向没有控制台的 Unity 输出日志流导致闪退
	nullFile, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0666)
	if err == nil {
		os.Stdout = nullFile
		os.Stderr = nullFile
	}

	ctx, cancel := context.WithCancel(context.Background())
	ctxCancel = cancel

	// 2. 使用当前分支真正的本地底层服务核心 daemon.New 
	instance, err := daemon.New(daemon.Options{
		Context:    ctx,
		ConfigPath: configPath,
	})
	if err != nil {
		cancel()
		return -1 // 配置路径不对或 JSON 格式错误，安全返回 -1，绝不闪退
	}

	// 3. 在独立协程中平滑拉起服务，防止阻塞 Unity 主线程
	go func() {
		defer func() { _ = recover() }()
		if err := instance.Start(); err != nil {
			// 启动失败安全消化
		}
	}()

	isRunning = true
	return 0
}

func stopInternal() {
	if ctxCancel != nil {
		ctxCancel() // 撤销上下文，daemon 内部会自动关闭所有的 Socket、Inbound 和 Outbound 链路
		ctxCancel = nil
	}
	isRunning = false
}

// ============================================================================
// CGO 导出函数 (Unity 调用的 C 接口)
// ============================================================================

//export StartSingBox
func StartSingBox(configPath *C.char) int {
	mu.Lock()
	defer mu.Unlock()
	defer func() { _ = recover() }()

	if isRunning {
		return 1
	}
	return startInternal(C.GoString(configPath))
}

//export ReloadSingBox
func ReloadSingBox(newConfigPath *C.char) int {
	mu.Lock()
	defer mu.Unlock()
	defer func() { _ = recover() }()

	stopInternal()
	return startInternal(C.GoString(newConfigPath))
}

//export StopSingBox
func StopSingBox() {
	mu.Lock()
	defer mu.Unlock()
	defer func() { _ = recover() }()

	stopInternal()
}
