package ai

import (
	"encoding/json"
)

// jsonFactors 将风险因子切片序列化为 JSON 字符串
func jsonFactors(factors []string) string {
	if len(factors) == 0 {
		factors = []string{"暂无显著风险因子"}
	}
	b, _ := json.Marshal(factors)
	return string(b)
}
