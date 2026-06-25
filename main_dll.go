package main

import "C"
import (
	"context"
	"sync"
	"github.com/mcqwyhud/sing-box-fork/box"
)

var (
	instance  *box.Box
	ctxCancel context.CancelFunc
	mu        sync.Mutex // 防止 Unity 多线程同时调用导致并发冲突
)

// 内部核心启动逻辑
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

// 内部核心停止逻辑
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

//export StartSingBox
// 首次启动时调用
func StartSingBox(configPath *C.char) int {
	mu.Lock()
	defer mu.Unlock()
	defer func() { recover() }()

	if instance != nil {
		return 1 // 已经运行中了
	}
	return startInternal(C.GoString(configPath))
}

//export ReloadSingBox
// 🔥 核心：热重载函数。当 s-ui 节点更新后，在 Unity 里直接调用此函数传入新配置
func ReloadSingBox(newConfigPath *C.char) int {
	mu.Lock()
	defer mu.Unlock()
	defer func() { recover() }()

	// 1. 优雅关闭旧内核，释放网络套接字和端口
	stopInternal()

	// 2. 紧接着拉起新内核
	return startInternal(C.GoString(newConfigPath))
}

//export StopSingBox
// 退出游戏或彻底关闭代理时调用
func StopSingBox() {
	mu.Lock()
	defer mu.Unlock()
	defer func() { recover() }()

	stopInternal()
}

func main() {}
