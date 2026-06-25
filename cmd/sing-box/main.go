//go:build !generate

package main

import "C"
import (
	"context"
	"os"
	"sync"
	"syscall"

	"github.com/sagernet/sing-box/log"
)

// 原版的 main 入口
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

// 针对 Windows 平台的硬重定向：修复没有标准控制台管道导致写日志 SIGPIPE 闪退的问题
func silenceSystemOutputs() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setStdHandle := kernel32.NewProc("SetStdHandle")
	
	// 打开一个空设备
	nullFile, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0666)
	if err == nil {
		handle := nullFile.Fd()
		// STD_OUTPUT_HANDLE = -11, STD_ERROR_HANDLE = -12
		_, _, _ = setStdHandle.Call(uintptr(1<<32-11), handle)
		_, _, _ = setStdHandle.Call(uintptr(1<<32-12), handle)
		os.Stdout = nullFile
		os.Stderr = nullFile
	}
}

func startInternal(configPath string) int {
	// 🔥 核心防闪退机制 A：拦截 Windows 系统底层所有的标准控制台输出句柄
	silenceSystemOutputs()

	ctx, cancel := context.WithCancel(context.Background())
	ctxCancel = cancel

	// 模拟官方原版 CLI 传参行为
	osArgs := []string{"sing-box", "run", "-c", configPath}
	
	// 🔥 核心防闪退机制 B：隔离线程并用最高级别的全局异常收容所
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// 强行吃掉一切 Go 侧的 Panic 崩溃，确保不波及宿主 Unity
			}
		}()
		
		mainCommand.SetArgs(osArgs[1:])
		
		// 使用带 Context 的原生引擎启动，如果遇到配置文件不存在、端口被抢占等
		// 如果它内部试图调用 os.Exit(1) 退出，由于没有直接暴露给系统，我们会尽可能用 Context 控制
		if err := mainCommand.ExecuteContext(ctx); err != nil {
			// 静默消化
		}
	}()

	isRunning = true
	return 0
}

func stopInternal() {
	if ctxCancel != nil {
		ctxCancel() // 触发 Context 撤销，平滑收回底层绑定的 Socket 监听
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
