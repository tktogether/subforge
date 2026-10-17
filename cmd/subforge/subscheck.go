package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tktogether/subforge/internal/subscheck"
)

func cmdSubsCheck(args []string) error {
	fs := flag.NewFlagSet("subscheck", flag.ContinueOnError)
	timeout := fs.Duration("timeout", 15*time.Second, "单 URL 探测超时")
	file := fs.String("file", "", "URL 列表文件（每行一个）")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var urls []string
	if *file != "" {
		f, err := os.Open(*file)
		if err != nil {
			return fmt.Errorf("打开文件 %s: %w", *file, err)
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			urls = append(urls, line)
		}
		if err := sc.Err(); err != nil {
			return err
		}
	} else {
		urls = fs.Args()
	}
	if len(urls) == 0 {
		return fmt.Errorf("未提供 URL（用 -file 或位置参数）")
	}
	results := subscheck.Check(context.Background(), urls, subscheck.Options{Timeout: *timeout})
	ok, fail := subscheck.Summary(results)
	for _, r := range results {
		status := "OK"
		if !r.OK {
			status = "FAIL"
		}
		fmt.Printf("%-4s %-60s %d %v", status, r.URL, r.StatusCode, r.Latency.Round(time.Millisecond))
		if r.Err != nil {
			fmt.Printf(" err=%v", r.Err)
		}
		fmt.Println()
	}
	fmt.Printf("\n合计：%d 成功，%d 失败\n", ok, fail)
	return nil
}
