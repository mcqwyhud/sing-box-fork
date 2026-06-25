//go:build !generate

package main

import "C" // 🔥 必须放在最顶部的 import 中
import (
	"context"
	"sync"

	"github.com/sagernet/sing-box/box"
	"github.com/sagernet/sing-box/log"
)

// ============================================================================
// Unity DLL 导出的全局变量
// ============================================================================
var (
	instance  *box.Box
	ctxCancel context.CancelFunc
	mu        sync.Mutex
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
// Unity DLL 内部核心逻辑
// ============================================================================
func startInternal(configPath string) int {
	ctx, cancel := context.WithCancel(context.Background())
	ctxCancel = cancel

	b, err := box.New(box.Options{
		Context:    ctx,
		ConfigPath: configPath,
	})
	if err != nil {
		return -1
	}

	instance = b
	err = instance.Start()
	if err != nil {
		return -2
	}
	return 0
}

func stopInternal() {
	if instance != nil {
		instance.Close()
		instance = nil
	}
	if ctxCancel != nil {
		ctxCancel()
		ctxCancel = nil
	}
}

// ============================================================================
// CGO 导出函数 (Unity 调用的接口)
// ============================================================================

//export StartSingBox
func StartSingBox(configPath *C.char) int {
	mu.Lock()
	defer mu.Unlock()
	defer func() { recover() }()

	if instance != nil {
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
	return startInternal(C.GoString(newConfigPath))
}

//export StopSingBox
func StopSingBox() {
	mu.Lock()
	defer mu.Unlock()
	defer func() { recover() }()

	stopInternal()
}
