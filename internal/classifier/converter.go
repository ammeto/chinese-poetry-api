package classifier

import (
	"fmt"

	"github.com/liuzl/gocc"
)

// s2t 与 t2s 在 init() 中初始化一次，可并发使用：
// 底层的 gocc.OpenCC.Convert 方法本身是并发安全的。
var (
	s2t *gocc.OpenCC // 简转繁
	t2s *gocc.OpenCC // 繁转简
)

func init() {
	var err error

	// 初始化简转繁转换器
	s2t, err = gocc.New("s2t")
	if err != nil {
		panic(fmt.Sprintf("failed to initialize s2t converter: %v", err))
	}

	// 初始化繁转简转换器
	t2s, err = gocc.New("t2s")
	if err != nil {
		panic(fmt.Sprintf("failed to initialize t2s converter: %v", err))
	}
}

// ToTraditional 把简体中文转为繁体。
func ToTraditional(text string) (string, error) {
	return s2t.Convert(text)
}

// ToSimplified 把繁体中文转为简体。
func ToSimplified(text string) (string, error) {
	return t2s.Convert(text)
}

// ToTraditionalArray 批量把字符串转为繁体。
func ToTraditionalArray(texts []string) ([]string, error) {
	result := make([]string, len(texts))
	for i, text := range texts {
		converted, err := ToTraditional(text)
		if err != nil {
			return nil, fmt.Errorf("failed to convert text at index %d: %w", i, err)
		}
		result[i] = converted
	}
	return result, nil
}

// ToSimplifiedArray 批量把字符串转为简体。
func ToSimplifiedArray(texts []string) ([]string, error) {
	result := make([]string, len(texts))
	for i, text := range texts {
		converted, err := ToSimplified(text)
		if err != nil {
			return nil, fmt.Errorf("failed to convert text at index %d: %w", i, err)
		}
		result[i] = converted
	}
	return result, nil
}

// ToTraditionalPointer 把字符串指针指向的内容转为繁体，nil 或空串原样返回。
func ToTraditionalPointer(text *string) (*string, error) {
	if text == nil || *text == "" {
		return text, nil
	}
	converted, err := ToTraditional(*text)
	if err != nil {
		return nil, err
	}
	return &converted, nil
}

// ConvertPoemToTraditional 把一首诗词的各个字段统一转为繁体。
func ConvertPoemToTraditional(title, author, content, rhythmic string) (string, string, string, string, error) {
	t, err := ToTraditional(title)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to convert title: %w", err)
	}

	a, err := ToTraditional(author)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to convert author: %w", err)
	}

	c, err := ToTraditional(content)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to convert content: %w", err)
	}

	r := rhythmic
	if rhythmic != "" {
		r, err = ToTraditional(rhythmic)
		if err != nil {
			return "", "", "", "", fmt.Errorf("failed to convert rhythmic: %w", err)
		}
	}

	return t, a, c, r, nil
}
