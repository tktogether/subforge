// subforge 是 SubForge 的命令行入口。
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"

	"github.com/tktogether/subforge/internal/buildinfo"
)

const usageText = `subforge - 订阅转换 + 分流规则一体化引擎（M1 开发中）

用法:
  subforge <命令> [参数]

命令:
  version    输出版本信息
  doctor     环境与运行时自检
  convert    本地把订阅转换成客户端配置
  diff       预览覆盖层对模板默认值的改动
  manifest   规则工厂 manifest 聚合（update/versions/rollback）
  mrs        MRS 离线生成（generate/cached）
  subscheck  订阅测活（默认关闭，需显式开启）
  scheduler  定时任务/告警（test-alert/run）
  serve      启动 HTTP 服务（M1-I5 增量实现）

运行 "subforge <命令> -h" 查看具体命令的参数。
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usageText)
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "version":
		err = cmdVersion(os.Args[2:])
	case "doctor":
		err = cmdDoctor(os.Args[2:])
	case "help", "-h", "--help":
		fmt.Print(usageText)
		return
	case "convert":
		err = cmdConvert(os.Args[2:])
	case "diff":
		err = cmdDiff(os.Args[2:])
	case "manifest":
		err = cmdManifest(os.Args[2:])
	case "mrs":
		err = cmdMRS(os.Args[2:])
	case "subscheck":
		err = cmdSubsCheck(os.Args[2:])
	case "scheduler":
		err = cmdScheduler(os.Args[2:])
	case "serve":
		err = cmdServe(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "未知命令 %q\n\n%s", os.Args[1], usageText)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "subforge: %v\n", err)
		os.Exit(1)
	}
}

func cmdVersion(args []string) error {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	fmt.Println(buildinfo.Full())
	return nil
}

func cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Println(buildinfo.Full())
	fmt.Printf("go:       %s\n", runtime.Version())
	fmt.Printf("platform: %s/%s (GOMAXPROCS=%d, NumCPU=%d)\n",
		runtime.GOOS, runtime.GOARCH, runtime.GOMAXPROCS(0), runtime.NumCPU())

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Printf("memory:   HeapAlloc=%.2f MB HeapSys=%.2f MB Sys=%.2f MB NumGC=%d\n",
		float64(ms.HeapAlloc)/(1<<20), float64(ms.HeapSys)/(1<<20),
		float64(ms.Sys)/(1<<20), ms.NumGC)

	if bi, ok := debug.ReadBuildInfo(); ok {
		fmt.Printf("module:   %s\n", bi.Main.Path)
		if rev := buildSetting(bi, "vcs.revision"); rev != "" {
			fmt.Printf("vcs:      %s (modified=%s)\n", rev, buildSetting(bi, "vcs.modified"))
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	fmt.Printf("workdir:  %s\n", wd)
	return nil
}

func buildSetting(bi *debug.BuildInfo, key string) string {
	for _, s := range bi.Settings {
		if s.Key == key {
			return s.Value
		}
	}
	return ""
}
