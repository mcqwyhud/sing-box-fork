package main // 🔥 保持和 cmd/sing-box/main.go 一致的包名

import "C"
import (
	"context"
	"sync"

	"github.com/sagernet/sing-box/box"
)

var (
	instance  *box.Box
	ctxCancel context.CancelFunc
	mu        sync.Mutex
)

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
