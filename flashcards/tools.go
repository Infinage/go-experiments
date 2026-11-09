//go:build tools

package tools

// Pinned in go.sum via this file so `go run github.com/go-task/task/v3/cmd/task`
// resolves without any global install and works offline after `go mod download`.
import _ "github.com/go-task/task/v3/cmd/task"
