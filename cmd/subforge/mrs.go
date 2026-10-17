package main

import (
	"flag"
	"fmt"

	"github.com/tktogether/subforge/internal/mrs"
)

func cmdMRS(args []string) error {
	if len(args) < 1 {
		fmt.Println("用法: subforge mrs <generate|cached> [参数]")
		fmt.Println("  generate <behavior> <format> <src> <dst>   生成 MRS 文件")
		fmt.Println("  cached <src> <cacheDir> <behavior> <format>   生成/命中缓存并打印字节数")
		return nil
	}
	switch args[0] {
	case "generate":
		return cmdMRSGenerate(args[1:])
	case "cached":
		return cmdMRSCached(args[1:])
	default:
		return fmt.Errorf("未知子命令 %q", args[0])
	}
}

func cmdMRSGenerate(args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("用法: subforge mrs generate <behavior> <format> <src> <dst>")
	}
	if err := mrs.GenerateFile(args[2], args[3], args[0], args[1]); err != nil {
		return err
	}
	fmt.Printf("OK %s -> %s\n", args[2], args[3])
	return nil
}

func cmdMRSCached(args []string) error {
	fs := flag.NewFlagSet("mrs cached", flag.ContinueOnError)
	ttl := fs.Duration("ttl", mrs.DefaultTTL, "缓存有效期")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 4 {
		return fmt.Errorf("用法: subforge mrs cached <src> <cacheDir> <behavior> <format>")
	}
	out, err := mrs.GenerateCached(fs.Arg(0), fs.Arg(1), fs.Arg(2), fs.Arg(3), *ttl)
	if err != nil {
		return err
	}
	fmt.Printf("%d bytes\n", len(out))
	return nil
}
