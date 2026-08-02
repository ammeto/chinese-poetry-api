package processor

import (
	"github.com/palemoky/chinese-poetry-api/internal/loader"
)

// PoemWork 是分发给 worker 的一条任务：原始诗词数据加上预分配的顺序 ID。
type PoemWork struct {
	loader.PoemWithMeta
	ID int64
}
