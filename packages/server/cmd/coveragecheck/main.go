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
	cmd := exec.Command("go", "tool", "cover", "-func="+profile)
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
