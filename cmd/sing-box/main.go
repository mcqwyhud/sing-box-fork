//go:build !generate

package main

import "C"
import (
	"context"
	"os"
	"sync"

	"github.com/sagernet/sing-box/log"
)

// ============================================================================
// 官方原版的 main 入口（保持原生逻辑，方便原本的编译体系）
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
	// 🔥 核心防闪退机制 1：将 Go 底层所有的标准输出和错误输出彻底重定向到系统的空设备
	// 防止 sing-box 往不存在的控制台管道（Stdout）写日志时触发 SIGPIPE 导致 Unity 闪退
	nullFile, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0666)
	if err == nil {
		os.Stdout = nullFile
		os.Stderr = nullFile
	}

	ctx, cancel := context.WithCancel(context.Background())
	ctxCancel = cancel

	// 模拟官方 CLI 的传参：sing-box run -c [配置文件路径]
	osArgs := []string{"sing-box", "run", "-c", configPath}
	
	// 🔥 核心防闪退机制 2：在独立的 Go 协程中启动服务，并用 recover 兜底
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// 捕获可能发生的 panic，防止它直接向上抛出导致 Unity 崩溃
			}
		}()
		mainCommand.SetArgs(osArgs[1:])
		
		// 使用带 Context 的原生核心引擎启动，这样外部调用 cancel() 时能做到真正平滑优雅退出
		if err := mainCommand.ExecuteContext(ctx); err != nil {
			// 静默处理错误，不调用任何会导致 os.Exit 的官方 Fatal 函数
		}
	}()

	isRunning = true
	return 0
}

func stopInternal() {
	if ctxCancel != nil {
		ctxCancel() // 触发 Context 撤销，通知底层所有绑定的 Socket 监听、TUN/TAP 设备平滑优雅释放
		ctxCancel = nil
	}
	isRunning = false
}

// ============================================================================
// CGO 导出函数 (供 Unity P/Invoke 直接调用的 C 风格标准接口)
// ============================================================================

//export StartSingBox
func StartSingBox(configPath *C.char) int {
	mu.Lock()
	defer mu.Unlock()
	
	// 终极防御：任何 CGO 边界函数必须自带 recover 守护
	defer func() { _ = recover() }()

	if isRunning {
		return 1 // 返回 1 代表服务已经在运行，拒绝重复拉起
	}
	return startInternal(C.GoString(configPath))
}

//export ReloadSingBox
func ReloadSingBox(newConfigPath *C.char) int {
	mu.Lock()
	defer mu.Unlock()
	defer func() { _ = recover() }()

	stopInternal()
	
	// 原地重启内核，实现网络核心规则、节点的毫秒级无缝热重载
	return startInternal(C.GoString(newConfigPath))
}

//export StopSingBox
func StopSingBox() {
	mu.Lock()
	defer mu.Unlock()
	defer func() { _ = recover() }()

	stopInternal()
}
