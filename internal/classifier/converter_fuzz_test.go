package classifier

import (
	"testing"
	"unicode/utf8"
)

// FuzzToTraditional 用随机输入对 ToTraditional 做模糊测试。
func FuzzToTraditional(f *testing.F) {
	// 用已知用例作为种子语料
	f.Add("简体中文")
	f.Add("床前明月光")
	f.Add("李白")
	f.Add("")
	f.Add("123abc!@#")
	f.Add("繁體中文") // Already traditional
	f.Add("混合text文字123")

	f.Fuzz(func(t *testing.T, input string) {
		// 函数不应 panic
		result, err := ToTraditional(input)

		// 合法的 UTF-8 字符串不应返回错误
		if utf8.ValidString(input) && err != nil {
			t.Errorf("ToTraditional(%q) returned unexpected error: %v", input, err)
		}

		// 结果应是合法的 UTF-8
		if !utf8.ValidString(result) {
			t.Errorf("ToTraditional(%q) returned invalid UTF-8: %q", input, result)
		}

		// 再转换回来应仍有结果（幂等性验证）
		if err == nil {
			backConverted, err2 := ToSimplified(result)
			if err2 != nil {
				t.Errorf("ToSimplified(ToTraditional(%q)) failed: %v", input, err2)
			}
			if !utf8.ValidString(backConverted) {
				t.Errorf("Round-trip conversion produced invalid UTF-8")
			}
		}
	})
}

// FuzzToSimplified 用随机输入对 ToSimplified 做模糊测试。
func FuzzToSimplified(f *testing.F) {
	// 用已知用例作为种子语料
	f.Add("繁體中文")
	f.Add("靜夜思")
	f.Add("李白")
	f.Add("")
	f.Add("123abc!@#")
	f.Add("简体中文") // Already simplified
	f.Add("混合text文字123")

	f.Fuzz(func(t *testing.T, input string) {
		// 函数不应 panic
		result, err := ToSimplified(input)

		// 合法的 UTF-8 字符串不应返回错误
		if utf8.ValidString(input) && err != nil {
			t.Errorf("ToSimplified(%q) returned unexpected error: %v", input, err)
		}

		// 结果应是合法的 UTF-8
		if !utf8.ValidString(result) {
			t.Errorf("ToSimplified(%q) returned invalid UTF-8: %q", input, result)
		}

		// 再转换回来应仍有结果（幂等性验证）
		if err == nil {
			backConverted, err2 := ToTraditional(result)
			if err2 != nil {
				t.Errorf("ToTraditional(ToSimplified(%q)) failed: %v", input, err2)
			}
			if !utf8.ValidString(backConverted) {
				t.Errorf("Round-trip conversion produced invalid UTF-8")
			}
		}
	})
}

// FuzzToTraditionalArray 对 ToTraditionalArray 做模糊测试。
func FuzzToTraditionalArray(f *testing.F) {
	// 种子语料
	f.Add("简体", "中文", "测试")
	f.Add("", "", "")
	f.Add("李白", "杜甫", "白居易")

	f.Fuzz(func(t *testing.T, s1, s2, s3 string) {
		input := []string{s1, s2, s3}

		// 不应 panic
		result, err := ToTraditionalArray(input)

		// 校验输入是否为合法 UTF-8
		allValid := true
		for _, s := range input {
			if !utf8.ValidString(s) {
				allValid = false
				break
			}
		}

		if allValid && err != nil {
			t.Errorf("ToTraditionalArray(%v) returned unexpected error: %v", input, err)
		}

		// 结果长度应与输入一致
		if err == nil && len(result) != len(input) {
			t.Errorf("ToTraditionalArray(%v) returned wrong length: got %d, want %d", input, len(result), len(input))
		}

		// 每一项结果都应是合法的 UTF-8
		if err == nil {
			for i, s := range result {
				if !utf8.ValidString(s) {
					t.Errorf("ToTraditionalArray(%v)[%d] returned invalid UTF-8: %q", input, i, s)
				}
			}
		}
	})
}
