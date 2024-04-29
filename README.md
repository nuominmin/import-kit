# batchimport

## 实现者

```go
type importService struct{}

type importContent struct {
	AAA uint64
	BBB string
	CCC string
	DDD string
	EEE string
	FFF string
}

func (p *importService) TransferStruct() interface{} {
	// 承载从excel解析数据后的传输结构
	return &importContent{}
}

func (p *importService) Start() error {
	// 请求导入中心，设置任务状态为开始
	return nil
}

func (p *importService) Progress() (doneInterval int, fn func(doneCount int) error) {
	// 返回每间隔 doneInterval 条完成记录进行一次回调一次
	// 用于导入任务进度更新
	return 200, func(done int) error {
		log.Info("import progress. done: %d", done)
		return nil
	}
}

func (p *importService) End(file *importkit.File, doneCount int, errorCount int) error {
	// 导入任务结束
	log.Info("import end. done count: %d, error count: %d", doneCount, errorCount)
	if errorCount == 0 {
		// todo: 更新任务状态为完成
		return nil
	}
	
	// todo: 错误处理
	// 1. 获取错误文件
	// 2. 上传错误文件
	// 3. 记录错误文件id和更新任务状态
    if errs.GetErrFile() == nil {
        return 0, nil
    }
    var buf *bytes.Buffer
    if buf, err = errs.GetErrFile().WriteToBuffer(); err != nil {
        return 0, err
    }
    if errFileID, _, err = storage.UploadFile(base64.StdEncoding.EncodeToString(buf.Bytes()), false); err != nil {
        return 0, err
    }
    return nil
}

func (p *importService) OpenFile() importkit.OpenFileFunc {
    // todo: 根据上传的文件ID打开文件
    // return importkit.OpenLocalFileFunc("test.xlsx")
    return func() (*importkit.File, error) {
		content, err := storage.DownloadFile(p.task.ImportFileId)
		if err != nil {
			return nil, err
		}
		return importkit.OpenBytesFile(content)
	}
}

func (p *importService) Save(rowsData importkit.Rows) error {
    // todo: 数据检验或数据导入
    
	log.Info("save start. ")

	for i := 0; i < len(rowsData); i++ {
		log.Info("row data: %+v", rowsData[i].Data)
	}
	log.Info("save end. ")
	return nil
}

```

## 简单导入
```go
package main

import (
	"fmt"
	importkit "github.com/nuominmin/import-kit"
)

var taskContainer importkit.TaskContainer
var localFilename = "test.xlsx" // 本地调试使用

func init() {
	taskContainer = importkit.NewService(&dependency{})
}

func main() {
	iImportService, err := taskContainer.NewImportTask(context.Background(), 1, &importService{}, 2)
	if err != nil {
		return
	}

	_ = iImportService.Run()
}

```

## 合并列数据导入
```go

iImportService, err := taskContainer.NewImportTask(context.Background(), 1, &importService{}, 2)
if err != nil {
	return 
}

if err := iImportService.SetUniqueColumn(0, 1);err != nil {
    fmt.Println(err.Error())
    return
}

_ = iImportService.Run()

```

## 附加列导入
```go

// 在实现者代码中，将承接数据的结构体里面继承 ExtraColumn 即可
type importContent struct {
    AAA uint64
    BBB string
    CCC string
    DDD string
    EEE string
    FFF string
    importkit.ExtraColumn                   // 附加列
}

```