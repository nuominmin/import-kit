package importkit

import (
	"context"
	"fmt"

	"github.com/nuominmin/import-kit/model"

	"github.com/nuominmin/import-kit/core"
	"github.com/nuominmin/import-kit/dependency"
)

type IImportImplementor interface {
	// TransferStruct 对应excel导入模板的中转结构体
	TransferStruct() interface{}
	// Submit 提交数据
	Submit(rows core.IRows, task *model.Task)
}

// TaskContainer 导入任务服容器
// 常驻内存
type TaskContainer interface {
	// NewImportTask 一个新的导入任务
	NewImportTask(ctx context.Context, taskId uint64, implementor IImportImplementor, skipRowNum int) (core.TaskScheduler, error)
	// NewImportCheckTask 一个新的导入检查任务
	NewImportCheckTask(openTplFileFunc, openImportFileFUnc core.OpenFileFunc, skipRowNum int, maxRowNum int64) core.ICheckService
}

type container struct {
	dependency dependency.Container
}

// importTask 每次都是一个新的任务
type importTask struct {
	ctx         context.Context      // 上下文
	task        *model.Task          // 导入任务数据
	dependency  dependency.Container // 外部依赖接口
	implementor IImportImplementor   // 实现类
}

// NewService .
func NewService(dependency dependency.Container) TaskContainer {
	return &container{
		dependency: dependency,
	}
}

func (s *container) NewImportTask(ctx context.Context, taskId uint64, implementor IImportImplementor, skipRowNum int) (core.TaskScheduler, error) {
	task, err := s.dependency.GetTaskById(ctx, taskId)
	if err != nil {
		return nil, err
	}

	return core.NewImportService(&importTask{
		ctx:         ctx,
		task:        task,
		dependency:  s.dependency,
		implementor: implementor,
	}, skipRowNum), nil
}

// TransferStruct 传输结构
func (it *importTask) TransferStruct() interface{} {
	return it.implementor.TransferStruct()
}

// Submit 调用实现者的 Submit 方法
func (it *importTask) Submit(rows core.IRows) {
	it.implementor.Submit(rows, it.task)
}

// OpenFile 打开文件
func (it *importTask) OpenFile() core.OpenFileFunc {
	return func() (*core.File, error) {
		content, err := it.dependency.DownloadFile(it.ctx, it.task.FileId)
		if err != nil {
			return nil, err
		}
		return core.OpenBytesFile(content)
	}
}

// Start 任务开始
func (it *importTask) Start() error {
	return it.dependency.TaskStart(it.ctx, it.task.ImportId)
}

// Progress 任务进度（每隔 doneInterval 秒将会执行 fn 一次）
func (it *importTask) Progress() (doneInterval int, fn func(total, doneCount int) error) {
	return 200, func(total, done int) error {
		return it.dependency.TaskProgressUpdate(it.ctx, it.task.ImportId, total, done)
	}
}

// End 任务结束, 更新成功状态 or 上传错误文件并更新错误文件ID和失败状态
func (it *importTask) End(errs core.IErrorMessages, doneCount int, errorCount int) error {
	if errorCount == 0 {
		return it.dependency.TaskSucceed(it.ctx, it.task.ImportId)
	}

	errFile := errs.GetErrFile()
	if errFile == nil {
		return nil
	}

	buffer, err := errFile.WriteToBuffer()
	if err != nil {
		return fmt.Errorf("[WriteToBuffer] error: %v", err)
	}

	var errFileId uint64
	if errFileId, err = it.dependency.UploadFile(it.ctx, buffer.Bytes()); err != nil {
		return fmt.Errorf("[UploadFile] error: %v", err)
	}

	if err = it.dependency.TaskFailed(it.ctx, it.task.ImportId, errFileId); err != nil {
		return fmt.Errorf("[TaskFailed] error: %v", err)
	}
	return nil
}

func (s *container) NewImportCheckTask(openTplFileFunc, openImportFileFUnc core.OpenFileFunc, skipRowNum int, maxRowNum int64) core.ICheckService {
	return core.NewCheckService(openTplFileFunc, openImportFileFUnc, skipRowNum, maxRowNum)
}
