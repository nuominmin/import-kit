package core

// ErrorWriteBackMode 错误回写模式
type ErrorWriteBackMode string

const (
	// ErrorWriteBackModeAnyRow 任意行：聚合后的行数据，任意行存在错误将会回写所有行到错误文件中
	ErrorWriteBackModeAnyRow ErrorWriteBackMode = "any"
	// ErrorWriteBackModeAssignRow 指定行：聚合后的行数据，只会将存在错误的行数据写入到错误文件中
	ErrorWriteBackModeAssignRow ErrorWriteBackMode = "assign"
)

// 错误回写模式是否为任意
func (e ErrorWriteBackMode) errorWriteBackModeIsAny() bool {
	return e == "" || e == ErrorWriteBackModeAnyRow
}
