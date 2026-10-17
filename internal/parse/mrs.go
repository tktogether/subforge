package parse

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"sort"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/metacubex/mihomo/component/cidr"
	"github.com/metacubex/mihomo/component/trie"
)

// MRS 是 mihomo 的二进制规则集格式（Meta Rule Set）。文件是 zstd 流，解压后依次是：
//
//	[4]byte 魔数 "MRS\x01"
//	[1]byte 匹配行为（0=domain，1=ipcidr，2=classical）
//	int64  规则条数（大端）
//	int64  扩展段长度（大端，当前恒为 0，留给后续版本）
//	扩展段
//	载荷：domain 是 succinct trie 的二进制快照，ipcidr 是 IP 区间数组
//
// 行为 2（classical）没有二进制形态——mihomo 只为 domain / ipcidr 实现了 FromMrs，
// 所以这里也只在 0 / 1 上工作。
//
// 解码只依赖内核的 component/trie 与 component/cidr，不引入 rules/provider（后者会
// 拖进整个 constant 包）。输出刻意与 mihomo 自带命令逐字节一致：
//
//	mihomo convert-ruleset <behavior> mrs <源文件> <目标文件>
//
// 这样"我们解出来的规则集"与"内核自己解出来的"可以直接 diff。

// mrsMagic 与内核 rules/provider.MrsMagicBytes 同值（MRSv1）。
var mrsMagic = [4]byte{'M', 'R', 'S', 1}

// zstdMagic 是 zstd 帧魔数；MRS 文件整体是 zstd 流，据此与文本规则集区分。
var zstdMagic = [4]byte{0x28, 0xb5, 0x2f, 0xfd}

// MRSBehavior 是 MRS 规则集的匹配行为。
type MRSBehavior int

const (
	// MRSBehaviorDomain 按域名匹配（载荷是域名集合）。
	MRSBehaviorDomain MRSBehavior = 0
	// MRSBehaviorIPCIDR 按网段匹配（载荷是 IP 区间集合）。
	MRSBehaviorIPCIDR MRSBehavior = 1
)

// String 返回行为名（日志与错误信息用）。
func (b MRSBehavior) String() string {
	switch b {
	case MRSBehaviorDomain:
		return "domain"
	case MRSBehaviorIPCIDR:
		return "ipcidr"
	default:
		return fmt.Sprintf("behavior(%d)", int(b))
	}
}

// MRSRuleSet 是解码后的 MRS 规则集。
type MRSRuleSet struct {
	// Behavior 是文件声明的匹配行为。
	Behavior MRSBehavior
	// Count 是文件头记录的规则条数（可能与 Rules 长度不同：域名集合会合并精确与后缀条目）。
	Count int
	// Rules 是 mihomo 文本规则集形态的条目，与 convert-ruleset 输出逐行一致。
	Rules []string
}

// IsMRS 判断内容是否是 MRS 二进制（只认 zstd 帧魔数，不解压）。
// 调用方用它把二进制规则集与文本规则集分流，避免把二进制当文本去校验。
func IsMRS(buf []byte) bool {
	return len(buf) >= len(zstdMagic) && bytes.Equal(buf[:len(zstdMagic)], zstdMagic[:])
}

// DecodeMRS 解码 MRS 二进制规则集。
// 返回的 Rules 与 mihomo convert-ruleset 的同行为输出逐行一致。
func DecodeMRS(buf []byte) (MRSRuleSet, error) {
	if !IsMRS(buf) {
		return MRSRuleSet{}, errors.New("不是 MRS 内容（缺少 zstd 帧魔数）")
	}
	zr, err := zstd.NewReader(bytes.NewReader(buf))
	if err != nil {
		return MRSRuleSet{}, fmt.Errorf("解压 MRS: %w", err)
	}
	defer zr.Close()

	var header [4]byte
	if _, err := io.ReadFull(zr, header[:]); err != nil {
		return MRSRuleSet{}, fmt.Errorf("读取 MRS 魔数: %w", err)
	}
	if header != mrsMagic {
		return MRSRuleSet{}, fmt.Errorf("MRS 魔数不匹配（读到 %q）", header[:])
	}

	var behavior [1]byte
	if _, err := io.ReadFull(zr, behavior[:]); err != nil {
		return MRSRuleSet{}, fmt.Errorf("读取 MRS 行为字节: %w", err)
	}

	var count int64
	if err := binary.Read(zr, binary.BigEndian, &count); err != nil {
		return MRSRuleSet{}, fmt.Errorf("读取 MRS 规则条数: %w", err)
	}
	if count < 0 {
		return MRSRuleSet{}, fmt.Errorf("MRS 规则条数非法: %d", count)
	}

	var extraLen int64
	if err := binary.Read(zr, binary.BigEndian, &extraLen); err != nil {
		return MRSRuleSet{}, fmt.Errorf("读取 MRS 扩展段长度: %w", err)
	}
	if extraLen < 0 {
		return MRSRuleSet{}, fmt.Errorf("MRS 扩展段长度非法: %d", extraLen)
	}
	if extraLen > 0 {
		if _, err := io.CopyN(io.Discard, zr, extraLen); err != nil {
			return MRSRuleSet{}, fmt.Errorf("跳过 MRS 扩展段: %w", err)
		}
	}

	out := MRSRuleSet{Count: int(count)}
	switch behavior[0] {
	case byte(MRSBehaviorDomain):
		out.Behavior = MRSBehaviorDomain
		set, err := trie.ReadDomainSetBin(zr)
		if err != nil {
			return MRSRuleSet{}, fmt.Errorf("解码 MRS 域名集合: %w", err)
		}
		out.Rules = dumpDomainSet(set)
	case byte(MRSBehaviorIPCIDR):
		out.Behavior = MRSBehaviorIPCIDR
		set, err := cidr.ReadIpCidrSet(zr)
		if err != nil {
			return MRSRuleSet{}, fmt.Errorf("解码 MRS 网段集合: %w", err)
		}
		set.Foreach(func(p netip.Prefix) bool {
			out.Rules = append(out.Rules, p.String())
			return true
		})
	default:
		return MRSRuleSet{}, fmt.Errorf("不支持的 MRS 行为 %d（只有 domain/ipcidr 有二进制形态）", behavior[0])
	}
	return out, nil
}

// dumpDomainSet 复刻内核 domainStrategy.DumpMrs 的输出：
// 同一个域名的"精确条目"与"后缀条目"合并成一条 "+.<domain>"，其余原样输出，
// 最后按完整文本排序。合并能把 992 条输入压成 992 条规则里的 `+.` 形态，
// 直接决定产物规则行数。
func dumpDomainSet(ds *trie.DomainSet) []string {
	if ds == nil {
		return nil
	}
	var keys []string
	ds.Foreach(func(key string) bool {
		keys = append(keys, key)
		return true
	})
	// 同一域名的两条条目必须相邻才能配对；按去掉前导 "." 的名字排序即可，
	// 且每个名字最多对应两条（集合内条目唯一）。
	sort.SliceStable(keys, func(i, j int) bool {
		return strings.Compare(strings.TrimPrefix(keys[i], "."), strings.TrimPrefix(keys[j], ".")) < 0
	})
	for i := 0; i+1 < len(keys); i++ {
		d := strings.TrimPrefix(keys[i], ".")
		if d == strings.TrimPrefix(keys[i+1], ".") {
			keys[i] = "+." + d
			keys[i+1] = ""
			i++
		}
	}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if k != "" {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
