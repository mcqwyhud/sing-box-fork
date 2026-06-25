//go:build !generate

package main

import "C"
import (
	"context"
	"os"
	"sync"

	"github.com/sagernet/sing-box/box"
	"github.com/sagernet/sing-box/log"
)

// 原版的 main 函数保留，仅为了通过编译器检查
func main() {
	if err := mainCommand.Execute(); err != nil {
		log.Fatal(err)
	}
}

var (
	instance  *box.Box // 🔥 直接托管底层的 Box 实例，不走命令行
	ctxCancel context.CancelFunc
	mu        sync.Mutex
)

func startInternal(configPath string) int {
	// 彻底杜绝控制台管道破裂闪退
	nullFile, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0666)
	if err == nil {
		os.Stdout = nullFile
		os.Stderr = nullFile
	}

	ctx, cancel := context.WithCancel(context.Background())
	ctxCancel = cancel

	// 🔥 降维打击：直接读取配置文件内容并用底层 API 创建实例
	// 这样可以完全绕过官方 mainCommand 里面各种致命的 os.Exit()
	b, err := box.New(box.Options{
		Context:    ctx,
		ConfigPath: configPath,
	})
	if err != nil {
		cancel()
		return -1 // 配置解析失败，安全返回错误码，绝不闪退
	}

	instance = b

	// 在独立线程启动服务
	go func() {
		defer func() { _ = recover() }()
		if err := instance.Start(); err != nil {
			// 启动失败静默处理
		}
	}()

	return 0
}

func stopInternal() {
	if instance != nil {
		_ = instance.Close() // 优雅关闭核心服务
		instance = nil
	}
	if ctxCancel != nil {
		ctxCancel()
		ctxCancel = nil
	}
}

// ============================================================================
// CGO 导出函数
// ============================================================================

//export StartSingBox
func StartSingBox(configPath *C.char) int {
	mu.Lock()
	defer mu.Unlock()
	defer func() { _ = recover() }()

	if instance != nil {
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
