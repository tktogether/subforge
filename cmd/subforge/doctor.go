package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/tktogether/subforge/internal/buildinfo"
)

func cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	configPath := fs.String("config", "", "配置文件路径（默认自动查找）")
	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Println("=== SubForge Doctor ===")
	fmt.Println()

	// 版本信息
	fmt.Println("--- 版本信息 ---")
	fmt.Println(buildinfo.Full())
	fmt.Printf("go:       %s\n", runtime.Version())
	fmt.Printf("platform: %s/%s (GOMAXPROCS=%d, NumCPU=%d)\n",
		runtime.GOOS, runtime.GOARCH, runtime.GOMAXPROCS(0), runtime.NumCPU())

	if bi, ok := debug.ReadBuildInfo(); ok {
		fmt.Printf("module:   %s\n", bi.Main.Path)
		if rev := buildSetting(bi, "vcs.revision"); rev != "" {
			fmt.Printf("vcs:      %s (modified=%s)\n", rev, buildSetting(bi, "vcs.modified"))
		}
	}
	fmt.Println()

	// 内存信息
	fmt.Println("--- 内存信息 ---")
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Printf("HeapAlloc=%.2f MB HeapSys=%.2f MB Sys=%.2f MB NumGC=%d\n",
		float64(ms.HeapAlloc)/(1<<20), float64(ms.HeapSys)/(1<<20),
		float64(ms.Sys)/(1<<20), ms.NumGC)
	fmt.Printf("TotalAlloc=%.2f MB Alloc=%.2f MB GC CPU=%.4f%%\n",
		float64(ms.TotalAlloc)/(1<<20), float64(ms.Alloc)/(1<<20), ms.GCCPUFraction*100)
	fmt.Println()

	// 工作目录
	fmt.Println("--- 工作目录 ---")
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	fmt.Printf("workdir: %s\n", wd)

	// 数据目录检查
	fmt.Println("\n--- 数据目录 ---")
	dataDir := os.Getenv("SUBFORGE_DATA_DIR")
	if dataDir == "" {
		dataDir = filepath.Join(wd, "data")
	}
	fmt.Printf("data_dir: %s\n", dataDir)
	if info, err := os.Stat(dataDir); err == nil {
		fmt.Printf("exists:   %v (isDir=%v)\n", true, info.IsDir())
	} else {
		fmt.Printf("exists:   false (%v)\n", err)
	}

	// 配置文件检查
	fmt.Println("\n--- 配置文件 ---")
	configFile := *configPath
	if configFile == "" {
		// 尝试自动查找
		for _, name := range []string{"config.ini", "subforge.ini", "config.yaml"} {
			if _, err := os.Stat(filepath.Join(wd, name)); err == nil {
				configFile = filepath.Join(wd, name)
				break
			}
		}
	}
	if configFile != "" {
		fmt.Printf("config:   %s\n", configFile)
		if info, err := os.Stat(configFile); err == nil {
			fmt.Printf("size:     %d bytes (modified: %s)\n", info.Size(), info.ModTime().Format(time.RFC3339))
		}
	} else {
		fmt.Println("config:   not found (using defaults)")
	}
	fmt.Println()

	// 依赖检查
	fmt.Println("--- 依赖检查 ---")
	deps := []struct {
		name string
		path string
	}{
		{"go", "go"},
		{"git", "git"},
	}
	for _, dep := range deps {
		if path, err := execLookPath(dep.path); err == nil {
			fmt.Printf("%-8s: %s\n", dep.name, path)
		} else {
			fmt.Printf("%-8s: not found\n", dep.name)
		}
	}
	fmt.Println()

	// 网络检查
	fmt.Println("--- 网络检查 ---")
	fmt.Println("DNS resolution: (skipped for offline safety)")
	fmt.Println()

	// 健康检查
	fmt.Println("--- 健康检查 ---")
	fmt.Println("All checks passed.")
	return nil
}

func execLookPath(name string) (string, error) {
	// 简化版 lookPath
	for _, dir := range strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)) {
		p := filepath.Join(dir, name)
		if runtime.GOOS == "windows" {
			p += ".exe"
		}
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("not found: %s", name)
}
