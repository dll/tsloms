// 跨平台覆盖率门禁检查工具
//
// 用法：
//
//	go run ./cmd/coveragecheck -threshold 80 -profile coverage.out
//
// 内部调用 go tool cover -func 生成合并后的 total 行，用浮点数与阈值比较，
// 避免旧 Makefile 依赖 grep/head/整数截断。
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func main() {
	threshold := flag.Float64("threshold", 80.0, "覆盖率门禁（百分比）")
	profile := flag.String("profile", "coverage.out", "go tool cover 生成的 profile 文件")
	flag.Parse()

	total, err := totalFromFunc(*profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("total coverage: %.2f%% (threshold: %.2f%%)\n", total, *threshold)
	if total < *threshold {
		fmt.Fprintf(os.Stderr, "ERROR: 覆盖率 %.2f%% 低于门禁 %.2f%%\n", total, *threshold)
		os.Exit(1)
	}
	fmt.Println("PASS: 覆盖率达标")
}

// totalFromFunc 通过 go tool cover -func=<profile> 解析合并后的 total 行。
func totalFromFunc(profile string) (float64, error) {
	normalized, err := normalizeProfile(profile)
	if err != nil {
		return 0, err
	}
	defer os.Remove(normalized)

	cmd := exec.Command("go", "tool", "cover", "-func="+normalized)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return 0, fmt.Errorf("go tool cover 失败: %v\n%s", ee, string(ee.Stderr))
		}
		return 0, fmt.Errorf("go tool cover 失败: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "total:") {
			continue
		}
		fields := strings.Fields(line)
		for _, fld := range fields {
			if !strings.HasSuffix(fld, "%") {
				continue
			}
			v, err := strconv.ParseFloat(strings.TrimSuffix(fld, "%"), 64)
			if err != nil {
				return 0, fmt.Errorf("解析覆盖率数值 %q 失败: %w", fld, err)
			}
			return v, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("未在 go tool cover 输出中找到 total 行")
}

// normalizeProfile 合并 go test -coverpkg=./... 生成的重复代码块。
// Go 会为每个被测包追加一份完整 profile，同一代码块可能出现多次；
// 直接交给 go tool cover 会只读取首份记录，造成覆盖率被系统性低估。
// 对同一代码块取最大执行次数，既能保留“是否覆盖”的事实，也不会重复累计次数。
func normalizeProfile(profile string) (string, error) {
	in, err := os.Open(profile)
	if err != nil {
		return "", fmt.Errorf("打开覆盖率文件失败: %w", err)
	}
	defer in.Close()

	type block struct {
		line  string
		count int
	}
	blocks := make(map[string]block)
	order := make([]string, 0)
	scanner := bufio.NewScanner(in)
	mode := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "mode:") {
			if mode == "" {
				mode = line
			}
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		count, err := strconv.Atoi(fields[2])
		if err != nil {
			return "", fmt.Errorf("解析覆盖率块 %q 失败: %w", line, err)
		}
		key := fields[0] + " " + fields[1]
		if old, ok := blocks[key]; !ok {
			blocks[key] = block{line: fields[0] + " " + fields[1] + " " + fields[2], count: count}
			order = append(order, key)
		} else if count > old.count {
			old.line = fields[0] + " " + fields[1] + " " + fields[2]
			old.count = count
			blocks[key] = old
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("读取覆盖率文件失败: %w", err)
	}

	out, err := os.CreateTemp("", "tsloms-coverage-*.out")
	if err != nil {
		return "", fmt.Errorf("创建临时覆盖率文件失败: %w", err)
	}
	name := out.Name()
	defer out.Close()
	if _, err := fmt.Fprintln(out, mode); err != nil {
		os.Remove(name)
		return "", fmt.Errorf("写入覆盖率模式失败: %w", err)
	}
	for _, key := range order {
		if _, err := fmt.Fprintln(out, blocks[key].line); err != nil {
			os.Remove(name)
			return "", fmt.Errorf("写入覆盖率块失败: %w", err)
		}
	}
	return name, nil
}
