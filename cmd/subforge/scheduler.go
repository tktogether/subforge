package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/tktogether/subforge/internal/scheduler"
)

func cmdScheduler(args []string) error {
	if len(args) < 1 {
		fmt.Println("用法: subforge scheduler <test-alert|run> [参数]")
		fmt.Println("  test-alert <channel> <target>   测试告警通道（telegram|dingtalk）")
		fmt.Println("  run                              启动调度器（前台，用于调试）")
		return nil
	}
	switch args[0] {
	case "test-alert":
		return cmdSchedulerTestAlert(args[1:])
	case "run":
		fmt.Println("调度器前台运行（Ctrl+C 退出）")
		s := scheduler.New()
		s.Start()
		select {}
	default:
		return fmt.Errorf("未知子命令 %q", args[0])
	}
}

func cmdSchedulerTestAlert(args []string) error {
	fs := flag.NewFlagSet("scheduler test-alert", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return fmt.Errorf("用法: subforge scheduler test-alert <telegram|dingtalk> <target>")
	}
	channel := fs.Arg(0)
	target := fs.Arg(1)
	ctx := context.Background()
	var ch scheduler.AlertChannel
	switch channel {
	case "telegram":
		// target = bot token; 第二参数从环境变量读取
		chatID := os.Getenv("TELEGRAM_CHAT_ID")
		if chatID == "" {
			return fmt.Errorf("请设置 TELEGRAM_CHAT_ID 环境变量")
		}
		ch = scheduler.NewTelegramChannel(target, chatID)
	case "dingtalk":
		ch = scheduler.NewDingTalkChannel(target)
	default:
		return fmt.Errorf("未知通道 %q（支持 telegram/dingtalk）", channel)
	}
	if err := ch.Send(ctx, "SubForge 告警测试：通道配置正常"); err != nil {
		return err
	}
	fmt.Println("告警发送成功")
	return nil
}
