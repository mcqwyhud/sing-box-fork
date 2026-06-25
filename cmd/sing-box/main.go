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

func main() {
	if err := mainCommand.Execute(); err != nil {
		log.Fatal(err)
	}
}

// ============================================================================
// Unity DLL 状态控制中心（引入初始化锁，攻克二次拉起闪退）
// ============================================================================
var (
	ctxCancel      context.CancelFunc
	mu             sync.Mutex
	isRunning      bool
	hasInitialized bool // 🔥 标记 Go 运行时是否在这个进程生命周期中初始化过
)

func silenceSystemOutputs() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setStdHandle := kernel32.NewProc("SetStdHandle")
	
	nullFile, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0666)
	if err == nil {
		handle := nullFile.Fd()
		_, _, _ = setStdHandle.Call(uintptr(1<<32-11), handle)
		_, _, _ = setStdHandle.Call(uintptr(1<<32-12), handle)
		os.Stdout = nullFile
		os.Stderr = nullFile
	}
}

func startInternal(configPath string) int {
	// 如果是第一次运行，执行底层的硬拦截绑定
	if !hasInitialized {
		silenceSystemOutputs()
		hasInitialized = true
	}

	ctx, cancel := context.WithCancel(context.Background())
	ctxCancel = cancel

	osArgs := []string{"sing-box", "run", "-c", configPath}
	
	go func() {
		defer func() { _ = recover() }()
		
		// 重置官方 mainCommand 的内部参数状态，防止二次运行时因残留上一次的参数导致硬报错
		mainCommand.SetArgs(osArgs[1:])
		
		if err := mainCommand.ExecuteContext(ctx); err != nil {
			// 静默消化
		}
	}()

	isRunning = true
	return 0
}

func stopInternal() {
	if ctxCancel != nil {
		ctxCancel() 
		ctxCancel = nil
	}
	isRunning = false
}

// ============================================================================
// CGO 导出函数
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
