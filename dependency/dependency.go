package dependency

import (
	"context"

	"github.com/nuominmin/import-kit/model"
)

// Container 依赖容器
type Container interface {
	// GetTaskById 根据ID获取导入任务
	GetTaskById(ctx context.Context, taskId uint64) (task *model.Task, err error)

	// DownloadFile 下载文件
	DownloadFile(ctx context.Context, fileId uint64) (content []byte, err error)

	// UploadFile 上传文件
	UploadFile(ctx context.Context, content []byte) (fileId uint64, err error)

	// TaskStart 任务开始
	TaskStart(ctx context.Context, taskId uint64) (err error)

	// TaskProgressUpdate 任务进度更新
	TaskProgressUpdate(ctx context.Context, taskId uint64, total, done int) (err error)

	// TaskSucceed 任务成功
	TaskSucceed(ctx context.Context, taskId uint64) (err error)

	// TaskFailed 任务失败
	TaskFailed(ctx context.Context, taskId, errFileId uint64) (err error)
}
