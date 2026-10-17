package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/tktogether/subforge/internal/manifest"
)

func cmdManifest(args []string) error {
	if len(args) < 1 {
		fmt.Println("用法: subforge manifest <update|versions|rollback> [参数]")
		fmt.Println("  update <manifest.yaml>   拉取所有 source 并写入版本化文件")
		fmt.Println("  versions <dir> <name> <ext>   列出某 source 的版本")
		fmt.Println("  rollback <dir> <name> <ext>   回滚到上一版并打印路径")
		return nil
	}
	switch args[0] {
	case "update":
		return cmdManifestUpdate(args[1:])
	case "versions":
		return cmdManifestVersions(args[1:])
	case "rollback":
		return cmdManifestRollback(args[1:])
	default:
		return fmt.Errorf("未知子命令 %q", args[0])
	}
}

func cmdManifestUpdate(args []string) error {
	fs := flag.NewFlagSet("manifest update", flag.ContinueOnError)
	retention := fs.Int("retention", manifest.DefaultRetention, "保留的最近版本数")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("用法: subforge manifest update <manifest.yaml>")
	}
	m, err := manifest.LoadManifest(fs.Arg(0))
	if err != nil {
		return err
	}
	results, err := m.Update(context.Background(), manifest.WithRetention(*retention))
	for _, r := range results {
		if r.Err != nil {
			fmt.Fprintf(os.Stderr, "FAIL  %s: %v\n", r.Source, r.Err)
		} else {
			fmt.Printf("OK    %s -> %s (version %s, fromCache=%v)\n", r.Source, r.Path, r.Version, r.FromCache)
		}
	}
	if err != nil {
		return err
	}
	return nil
}

func cmdManifestVersions(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("用法: subforge manifest versions <dir> <name> <ext>")
	}
	versions, err := manifest.ListVersions(args[0], args[1], args[2])
	if err != nil {
		return err
	}
	for _, v := range versions {
		fmt.Println(v)
	}
	return nil
}

func cmdManifestRollback(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("用法: subforge manifest rollback <dir> <name> <ext>")
	}
	p, err := manifest.Rollback(args[0], args[1], args[2])
	if err != nil {
		return err
	}
	fmt.Println(p)
	return nil
}
