//go:build tools
// +build tools

// Package tools 固定项目所需的工具类依赖，
// 以保证 `go mod tidy` 不会把它们从 go.mod 中移除。
// 参见：https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
package tools

import (
	_ "github.com/99designs/gqlgen"
)
