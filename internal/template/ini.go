// Package template 解析 subconverter 风格的 INI 模板。
//
// 本文件只做"结构解析 + !include 展开"，不解释任何 key 的含义——语义层在
// template 包的其余文件里。这样覆盖优先级链可以纯靠"按顺序 Overlay、后者
// 覆盖前者"实现，解析层不需要知道业务规则。
package template

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// groupSectionName 是允许使用 "---" 分隔符的段落名（大小写不敏感比较）。
const groupSectionName = "clash_proxy_group"

// GroupSectionName 是代理组段落的规范名，供语义层按段落名做特判。
const GroupSectionName = groupSectionName

// groupSeparator 是 "---" 分隔符在条目流里的内部键名。
// 用不可打印字符开头，用户写不出这个键，不会与真实配置冲突。
const groupSeparator = "\x00group-sep"

// isGroupSection 判断段落是否允许"同名键重复 + --- 分隔多个组"的写法。
func isGroupSection(name string) bool {
	return strings.EqualFold(name, groupSectionName)
}

// repeatable 是允许在同一段里重复出现的键。
//
// subconverter 的模板把分组和规则一行一条地写（[custom] 里的 custom_proxy_group=
// 出现几十次），按"同名键后者覆盖前者"解析的话只剩最后一条，整份模板的分组与规则
// 都会消失。这些键必须逐条追加，顺序还是语义的一部分。
var repeatable = map[string]bool{
	"custom_proxy_group": true,
	"ruleset":            true,
	"rules":              true,
	// [pipeline:<name>] 里一行一步，同名键必须逐条追加，否则只剩最后一步。
	"step": true,
	// override INI 里一组多成员写出。
	"proxy":   true,
	"prepend": true,
	"append":  true,
}

func isRepeatable(key string) bool {
	return repeatable[strings.ToLower(strings.TrimSpace(key))]
}

// Value 是一个配置值，带上来源信息。
// 来源可追溯是排查"这个设置到底来自哪个文件"的刚需。
type Value struct {
	// Raw 原始文本（去掉了首尾空白）。
	Raw string
	// Source 定义所在文件；!include 展开后指向实际定义处，而不是引用处。
	Source string
	// Line 定义所在行号（从 1 开始）。
	Line int
}

// Entry 是一段中的一条键值对。
type Entry struct {
	Key   string
	Value Value
}

// Section 是一个段落，条目按出现顺序保存。
// 单一切片同时给出"顺序"与"取值"，比 map + 顺序表少一份需要同步的状态；
// 段落规模很小（几十条键），线性查找的代价可以忽略。
type Section struct {
	Name    string
	entries []Entry
}

// Get 取一个键的值；同名键出现多次时返回最后一次设置的值。
func (s *Section) Get(key string) (Value, bool) {
	if s == nil {
		return Value{}, false
	}
	for i := len(s.entries) - 1; i >= 0; i-- {
		if s.entries[i].Key == key {
			return s.entries[i].Value, true
		}
	}
	return Value{}, false
}

// Has 判断键是否已设置。
func (s *Section) Has(key string) bool {
	_, ok := s.Get(key)
	return ok
}

// Keys 按首次出现顺序返回键名（同名键只出现一次）。
func (s *Section) Keys() []string {
	seen := make(map[string]bool, len(s.entries))
	out := make([]string, 0, len(s.entries))
	for _, e := range s.entries {
		if !seen[e.Key] {
			seen[e.Key] = true
			out = append(out, e.Key)
		}
	}
	return out
}

// Entries 按出现顺序返回全部条目（含同名键的重复项）。
func (s *Section) Entries() []Entry {
	out := make([]Entry, len(s.entries))
	copy(out, s.entries)
	return out
}

// Set 设置一个键：已存在则原地覆盖值并保留位置，否则追加到末尾。
func (s *Section) Set(key string, v Value) {
	for i := len(s.entries) - 1; i >= 0; i-- {
		if s.entries[i].Key == key {
			s.entries[i].Value = v
			return
		}
	}
	s.entries = append(s.entries, Entry{Key: key, Value: v})
}

// Add 无条件追加一条条目，不去重同名键。
// [clash_proxy_group] 段落里的 "---" 分隔符出现多次也必须被完整保留，
// 否则多代理组的边界信息会丢——Set 做不到这一点。
func (s *Section) Add(key string, v Value) {
	s.entries = append(s.entries, Entry{Key: key, Value: v})
}

// Replace 先删掉同名键的全部条目，再追加这一条。
// 覆盖合并遇到可重复键时用它：后合入的模板声明了自己的分组清单，就应该整份取代，
// 而不是只改其中一条。
func (s *Section) Replace(key string, v Value) {
	for i := 0; i < len(s.entries); {
		if strings.EqualFold(s.entries[i].Key, key) {
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
			continue
		}
		i++
	}
	s.entries = append(s.entries, Entry{Key: key, Value: v})
}

// Blocks 按分隔键把条目切成若干块，分隔符本身不出现在结果里。
func (s *Section) Blocks(sep string) [][]Entry {
	blocks := [][]Entry{nil}
	for _, e := range s.entries {
		if e.Key == sep {
			blocks = append(blocks, nil)
			continue
		}
		blocks[len(blocks)-1] = append(blocks[len(blocks)-1], e)
	}
	// 连续分隔符、尾部分隔符都会产生空块，去掉。
	out := blocks[:0]
	for _, b := range blocks {
		if len(b) > 0 {
			out = append(out, b)
		}
	}
	return out
}

// GroupBlocks 按 "---" 分隔符切块，专用于 [clash_proxy_group]：
// subconverter 用它在一个段落里定义多个代理组。其它段落用不到。
func (s *Section) GroupBlocks() [][]Entry {
	return s.Blocks(groupSeparator)
}

// AddGroupSeparator 在段落末尾追加一个 "---" 分隔符。
// 供语义层合并两个模板的代理组时用：直接追加条目会丢掉组之间的边界。
func (s *Section) AddGroupSeparator() {
	s.Add(groupSeparator, Value{})
}

// Config 是解析结果。
type Config struct {
	sections map[string]*Section
	order    []string
}

// Section 取段落；不存在时创建。惰性创建让调用方少写判断。
func (c *Config) Section(name string) *Section {
	if s, ok := c.sections[name]; ok {
		return s
	}
	s := &Section{Name: name}
	c.sections[name] = s
	c.order = append(c.order, name)
	return s
}

// Lookup 取段落，不存在时返回 nil 而不创建。
func (c *Config) Lookup(name string) *Section {
	return c.sections[name]
}

// Has 判断段落是否存在。
func (c *Config) Has(name string) bool {
	_, ok := c.sections[name]
	return ok
}

// SectionNames 按出现顺序返回段落名。
func (c *Config) SectionNames() []string {
	out := make([]string, len(c.order))
	copy(out, c.order)
	return out
}

// Get 便捷取某个段落里的某个键。
func (c *Config) Get(section, key string) (Value, bool) {
	return c.Lookup(section).Get(key)
}

// Scalar 取段落里某个键的原始字符串，未设置返回 fallback。
func (c *Config) Scalar(section, key, fallback string) string {
	if v, ok := c.Lookup(section).Get(key); ok {
		return v.Raw
	}
	return fallback
}

// Overlay 把 other 合进来，同名段落与同名键都由 other 覆盖。
// 覆盖优先级链就是靠它实现的：先合低优先级，再合高优先级。
//
// 注意：覆盖合并只能"替换"，不能"相加"。[clash_proxy_group] 里两个模板各有多组时，
// 这里只会留下后者的组。需要两组代理组相加的场景走 config.Merge，它对该段落按块追加。
func (c *Config) Overlay(other *Config) {
	c.OverlayExcept(other, nil)
}

// OverlayExcept 同 Overlay，但跳过 skip 判定为真的段落。
// [clash_proxy_group] 需要按"组"追加而不是按键覆盖，走这个入口。
// 可重复键走 Replace：整份清单由后者取代，而不是只改掉其中一条。
func (c *Config) OverlayExcept(other *Config, skip func(name string) bool) {
	if other == nil {
		return
	}
	for _, name := range other.order {
		if skip != nil && skip(name) {
			continue
		}
		target := c.Section(name)
		for _, e := range other.sections[name].Entries() {
			if isRepeatable(e.Key) {
				target.Replace(e.Key, e.Value)
				continue
			}
			target.Set(e.Key, e.Value)
		}
	}
}

// Clone 深拷贝一份配置：段落与条目按原顺序复制，包括重复的分隔符条目。
func (c *Config) Clone() *Config {
	out := &Config{sections: map[string]*Section{}}
	if c == nil {
		return out
	}
	for _, name := range c.order {
		s := &Section{Name: name}
		s.entries = append(s.entries, c.sections[name].Entries()...)
		out.sections[name] = s
		out.order = append(out.order, name)
	}
	return out
}

// Parse 解析一段 INI 文本。origin 是来源标识，用于错误信息与 Value.Source；
// 文本里出现 !include 时需要 baseDir 才能解析相对路径。
func Parse(data []byte, origin string) (*Config, error) {
	return parseBytes(data, origin, "", nil)
}

// ParseString 是 Parse 的字符串版本。
func ParseString(s, origin string) (*Config, error) {
	return Parse([]byte(s), origin)
}

// ParseFile 解析文件并展开其中的 !include / !!import。
func ParseFile(path string) (*Config, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	return parseFileRecursive(abs, []string{abs})
}

// parseFileRecursive 读取一个绝对路径文件并解析，栈用于循环引用检测。
func parseFileRecursive(abs string, stack []string) (*Config, error) {
	buf, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("读取配置 %s: %w", abs, err)
	}
	return parseBytes(buf, filepath.Base(abs), filepath.Dir(abs), stack)
}

// parseBytes 是解析主体。
//
// label 用于错误信息；baseDir 是 !include 相对路径的解析基准（解析纯文本时为空）；
// stack 是当前文件链（含自身），用于循环引用检测。
func parseBytes(data []byte, label, baseDir string, stack []string) (*Config, error) {
	cfg := &Config{sections: map[string]*Section{}}
	var cur *Section

	for n, raw := range strings.Split(string(data), "\n") {
		lineNo := n + 1
		// 尾部空格可能是配置者有意保留的——[node_pref] 的前缀就靠它和节点名隔开。
		// 所以整行只去掉 '\r' 与注释；line 是去空格后的"结构性"视图，
		// 用来判空行、段落头、分隔符；键值拆分保留原始尾部空格。
		stripped := stripComment(strings.TrimRight(raw, "\r"))
		line := strings.TrimSpace(stripped)
		if line == "" {
			continue
		}

		if name, ok := splitSection(line); ok {
			cur = cfg.Section(name)
			continue
		}

		if target, ok := splitInclude(line, cur == nil); ok {
			abs, err := resolveInclude(baseDir, target)
			if err != nil {
				return nil, fmt.Errorf("%s:%d !include: %w", label, lineNo, err)
			}
			if inStack(stack, abs) {
				return nil, fmt.Errorf("%s:%d !include: 循环引用 %s", label, lineNo, abs)
			}
			other, err := parseFileRecursive(abs, append(append([]string{}, stack...), abs))
			if err != nil {
				return nil, err
			}
			if cur == nil {
				// 顶层 !include：整份配置合并进来。
				cfg.Overlay(other)
			} else {
				// 段内 !include：只合入同名段落，不能把整份配置灌进当前段。
				if otherSec := other.Lookup(cur.Name); otherSec != nil {
					for _, e := range otherSec.Entries() {
						cur.Set(e.Key, e.Value)
					}
				}
			}
			continue
		}

		// [clash_proxy_group] 里单独一行的 "---" 是代理组分隔符，不是一条键值对；
		// 其它段落里出现仍然按语法错误处理，避免悄悄吞掉用户的拼写错误。
		if line == "---" && cur != nil && isGroupSection(cur.Name) {
			cur.Add(groupSeparator, Value{Raw: "", Source: label, Line: lineNo})
			continue
		}

		key, value, ok := splitKV(strings.TrimLeft(stripped, " \t"))
		if !ok {
			return nil, fmt.Errorf("%s:%d: 无法解析的行（应为 key = value）: %q", label, lineNo, line)
		}
		if cur == nil {
			return nil, fmt.Errorf("%s:%d: 键 %q 出现在任何段落之前", label, lineNo, key)
		}
		v := Value{Raw: value, Source: label, Line: lineNo}
		if isGroupSection(cur.Name) || isRepeatable(key) {
			// [clash_proxy_group] 里多个组各有各的 name/type，同名键必须追加而不是
			// 原地覆盖——Set 会让第二个组的键写回第一个组的位置，块边界就此消失。
			// 可重复键同理：模板一行一条地声明分组与规则，覆盖语义只剩最后一条。
			cur.Add(key, v)
		} else {
			cur.Set(key, v)
		}
	}
	return cfg, nil
}

// stripComment 去掉注释。
//
// '#' 在任意位置起注释；';' 只在行首起注释——因为值里合法出现分号
// （rename / emoji 规则用分号分隔多条）。
func stripComment(line string) string {
	if i := strings.Index(line, "#"); i >= 0 {
		line = line[:i]
	}
	if strings.HasPrefix(strings.TrimLeft(line, " \t"), ";") {
		return ""
	}
	return line
}

// splitSection 识别 "[段落名]" 形式的行。
func splitSection(line string) (string, bool) {
	if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
		return "", false
	}
	name := strings.TrimSpace(line[1 : len(line)-1])
	if name == "" {
		return "", false
	}
	return name, true
}

// includeDirectives 是会被当成指令处理的行首关键字。
// 注意 '!emoji'、'!remote' 之类在 [common] 里是普通键，不能被误认成指令。
var includeDirectives = []string{"!!import", "!import", "!include"}

// splitInclude 识别 "指令 + 路径" 形式的行，返回路径。
//
// top 表示当前是否在任何段落之前。"!include" 只在顶层当成指令——subconverter 的
// [common] 段落里 !include / !exclude 是节点名过滤的合法键名，在段落内误判会把用户
// 的正则当成文件路径去读。"!import" 与 "!!import" 没有这个冲突，任何层级都认。
func splitInclude(line string, top bool) (string, bool) {
	lower := strings.ToLower(line)
	for _, d := range includeDirectives {
		if d == "!include" && !top {
			continue
		}
		if strings.HasPrefix(lower, d) {
			rest := line[len(d):]
			if rest == "" || rest[0] == ' ' || rest[0] == '\t' {
				return strings.TrimSpace(rest), true
			}
		}
	}
	return "", false
}

// splitKV 按第一个 '=' 或 ':' 拆成键值；'=' 优先。
//
// 值只去首部空格：尾部空格可能是配置者有意保留的（[node_pref] 的前缀），
// 需要它的地方自己去 TrimSpace。
func splitKV(line string) (string, string, bool) {
	for _, sep := range []string{"=", ":"} {
		if i := strings.Index(line, sep); i > 0 {
			key := strings.TrimSpace(line[:i])
			if key == "" {
				return "", "", false
			}
			return key, strings.TrimLeft(line[i+1:], " \t"), true
		}
	}
	return "", "", false
}

// resolveInclude 把 !include 的目标解析成存在的绝对路径。
func resolveInclude(baseDir, target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("缺少文件路径")
	}
	p := target
	if !filepath.IsAbs(p) {
		if baseDir == "" {
			return "", fmt.Errorf("相对路径 %q 无法解析：没有基准目录", target)
		}
		p = filepath.Join(baseDir, target)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	if st, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("找不到 %s: %w", abs, err)
	} else if st.IsDir() {
		return "", fmt.Errorf("%s 是目录不是文件", abs)
	}
	return abs, nil
}

func inStack(stack []string, abs string) bool {
	for _, p := range stack {
		if p == abs {
			return true
		}
	}
	return false
}
