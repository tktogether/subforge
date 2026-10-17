package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/tktogether/subforge/internal/rulebot"
)

func cmdRuleBot(args []string) error {
	if len(args) < 1 {
		fmt.Println("用法: subforge rulebot <analyze|pending|stats> [参数]")
		fmt.Println("  analyze <file>   分析日志文件，输出检测到的异常与建议")
		fmt.Println("  pending          列出待确认的建议")
		fmt.Println("  stats            输出异常类型统计")
		return nil
	}
	switch args[0] {
	case "analyze":
		return cmdRuleBotAnalyze(args[1:])
	case "stats":
		return cmdRuleBotStats(args[1:])
	default:
		return fmt.Errorf("未知子命令 %q", args[0])
	}
}

func cmdRuleBotAnalyze(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("用法: subforge rulebot analyze <log-file>")
	}
	detector := rulebot.NewDetector()
	queue := rulebot.NewPendingQueue()
	f, err := os.Open(args[0])
	if err != nil {
		return fmt.Errorf("打开日志文件 %s: %w", args[0], err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	count := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		a, sug := detector.Analyze(line, args[0])
		if a != nil && sug != nil {
			count++
			fmt.Printf("[%s] %s\n  建议: %s %s\n  理由: %s\n\n",
				a.Type, a.Message, sug.Action, sug.Rule, sug.Reason)
			queue.Enqueue(sug)
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	fmt.Printf("检测到 %d 条异常\n", count)
	return nil
}

func cmdRuleBotStats(args []string) error {
	// 无持久化时仅输出说明
	fmt.Println("异常统计需先 analyze 分析日志后获取")
	fmt.Println("用法: subforge rulebot analyze <log-file>")
	return nil
}
