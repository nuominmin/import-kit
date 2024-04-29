package model

// Task 导入任务
type Task struct {
	ImportId   uint64 // 导入中心id
	FileId     uint64
	Status     string
	Params     string
	OperatorId uint64
}
