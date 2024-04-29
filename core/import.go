package core

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

type TaskScheduler interface {
	// SetUniqueColumn 设置唯一列进行行数据聚合
	SetUniqueColumn(indexes ...int) error
	// SetErrorWriteBackMode 设置错误会写模式
	SetErrorWriteBackMode(mode ErrorWriteBackMode)
	// Run 运行扫描任务。 这是一个阻塞的方法，将会扫描全表后才退出
	Run() error
}

// ImplementorContainer API请求或方法调用者需要实现的方法
type ImplementorContainer interface {
	// Start 开始
	Start() error
	// End 结束
	End(errs IErrorMessages, doneCount int, errorCount int) error
	// Progress 进度更新，doneInterval 完成间隔
	Progress() (doneInterval int, fn func(total, doneCount int) error)
	// OpenFile 打开文件
	OpenFile() OpenFileFunc
	// TransferStruct 对应excel导入模板的中转结构体
	TransferStruct() interface{}
	// Submit 提交数据
	Submit(rows IRows)
}

// 导入任务服务的结构体
type taskScheduler struct {
	id              string
	rows            [][]string
	rowsCount       int                  // 总行数
	skipRowNum      int                  // 跳过行数。头部行的数量
	sheetName       string               // 工作表
	transferStruct  *transferStruct      // 转换结构字段数量
	maxColumnNum    int                  // 列最大列数，取决于最大的字段数量和行数据列数，主要用于写入错误列时追加最到最后一列； 只有两种情况会写入这个值： 1. 初始化时的 transferStruct.Num, 2. 附加列时的最大列
	implementor     ImplementorContainer // 实现类
	groupRows       *groupRows
	headerFirstData *headerFirstData
}

func NewImportService(implementor ImplementorContainer, skipRowNum int) TaskScheduler {
	if skipRowNum <= 0 {
		skipRowNum = 2
	}
	id := strings.ReplaceAll(uuid.New().String(), "-", "")
	svc := &taskScheduler{
		id:             id,
		skipRowNum:     skipRowNum,
		sheetName:      "",
		transferStruct: &transferStruct{},
		maxColumnNum:   0,
		implementor:    implementor,
		groupRows: &groupRows{
			rows:               []*uniqueColumn{},
			keyMapIndex:        make(map[string]int),
			indexes:            []int{},
			indexesLen:         0,
			errorWriteBackMode: ErrorWriteBackModeAnyRow,
		},
		headerFirstData: &headerFirstData{},
	}
	return svc
}

type transferStruct struct {
	fieldNum int // 字段数量
	typeOf   reflect.Type
}

type groupRows struct {
	rows               []*uniqueColumn    // 唯一列分组行
	keyMapIndex        map[string]int     // map[key]group index, key: row data value
	indexes            []int              // 唯一列索引
	indexesLen         int                // 唯一列索引长度
	errorWriteBackMode ErrorWriteBackMode // 错误写回模式
}

type uniqueColumn struct {
	rowsData IRows // 行数据
	mergeNum int   // 合并数量
}

// 表头第一行数据
type headerFirstData struct {
	num  int      // 列数
	data []string // 数据
}

// 输出错误。输出的id主要用于搜索错误日志的时候可以通过标识过滤出同一个任务错误
func (p *taskScheduler) outputError(format string, a ...interface{}) {
	fmt.Printf("id: %s, error: %s", p.id, fmt.Sprintf(format, a...))
}

func (p *taskScheduler) Run() error {
	if p.implementor == nil {
		return errors.New("implementor is empty. ")
	}

	err := p.implementor.Start()
	if err != nil {
		p.outputError("start error: %+v", err)
		return err
	}

	f := &File{}
	f, err = p.implementor.OpenFile()()
	if err != nil {
		p.outputError("open file error: %+v", err)
		return err
	}

	defer func() {
		if f == nil {
			return
		}
		if err1 := f.Close(); err1 != nil {
			p.outputError("close file error: %+v", err1)
		}
	}()

	// 设置转换结构字段数量
	if p.transferStruct.fieldNum == 0 {
		p.setTransferStruct()
	}

	// 读取行数据
	if err = p.readRows(f); err != nil {
		p.outputError("read rows error: %+v", err)
		return err
	}

	// 关闭文件对象
	_ = f.Close()
	f = nil

	// 可能是个空文件
	if p.rowsCount <= p.skipRowNum {
		p.outputError("this is empty file. ")
		return nil
	}

	// 设置唯一列映射行数量
	if err = p.setUniqueColumnMapRowNum(); err != nil {
		p.outputError("set unique column map num error: %+v", err)
		return err
	}

	// 记录表头信息, 因为上面判断了行数不能小于等于跳过行数，所以这里直接用 0 取值
	p.headerFirstData.data = p.rows[0]
	p.headerFirstData.num = len(p.rows[0])

	// 空行
	emptyRowNum := 0

	// 错误消息
	errMessages := newErrorMessages()

	// 完成行数
	doneCount := 0

	for i := p.skipRowNum; i < p.rowsCount; i++ {
		// 是空行则跳过
		if isEmpty(p.rows[i]) {
			emptyRowNum++
			// 空行不处理，不认为是失败操作
			continue
		}

		// 解析数据
		iRowData := p.parseRowData(p.rows[i])

		// rowsData：即将被提交的行数据
		var rowsData IRows

		// 获取唯一列的key
		if index, ok := p.getUniqueColumnIndex(p.rows[i]); ok {
			// 暂存 row 到 groupRows 中
			// 减少需要合并的行数据量
			p.groupRows.rows[index].rowsData.Append(iRowData, i)
			p.groupRows.rows[index].mergeNum--

			// 判断该行数据是否可以封包，不可以则继续等待封包
			if p.groupRows.rows[index].mergeNum > 0 {
				continue
			}

			// 当前需要合并的行数据是0
			// 取出暂存在 groupRows 中的 rows
			rowsData = p.groupRows.rows[index].rowsData
			p.groupRows.rows[index] = nil
		} else {
			rowsData = newRows() // 初始化为空的
			rowsData.Append(iRowData, i)
		}

		/*
			场景：
				1. 逐行提交	-> SubmitForEach -> error SetRowErr
				2. 一次性提交	-> Submit -> error SetRowErr
		*/

		// 提交数据
		p.implementor.Submit(rowsData)

		// 从已提交的 rows 中尝试获取错误消息并写入到 errs
		if rowsData.IsErr() {
			for j := 0; j < rowsData.Count(); j++ {
				rowData := rowsData.GetRow(j)
				formIndex := rowData.GetFormIndex()
				var printErr error
				if errs := rowData.GetErrs(); len(errs) > 0 {
					printErr = errs.PrintError()
					p.outputError("Submit error: %+v, form index: %d", printErr, formIndex)
				}
				if p.groupRows.errorWriteBackMode.errorWriteBackModeIsAny() || printErr != nil {
					errMessages.Append(formIndex, printErr)
				}
			}
		}

		doneCount = i - p.skipRowNum - emptyRowNum // 完成行数
		doneInterval, fn := p.implementor.Progress()
		if doneInterval <= 0 {
			doneInterval = 100 // 默认 100
		}
		if fn != nil && doneCount%doneInterval == 0 {
			if err = fn(p.rowsCount, doneCount); err != nil {
				p.outputError("heart beat error: %+v", err)
			}
		}
	}

	doneCount = p.rowsCount - p.skipRowNum - emptyRowNum // 实际完成行数
	errorCount := errMessages.Count()                    // 错误行数

	err = errMessages.Build(p.rows, p.maxColumnNum, p.skipRowNum)
	if err != nil {
		p.outputError("write error: %+v", err)
	}

	err = p.implementor.End(errMessages, doneCount, errorCount)
	if err != nil {
		p.outputError("start error: %+v", err)
		return err
	}

	return nil
}

// 是否开启分组行
func (p *taskScheduler) isEnableGroupRows() bool {
	return p.groupRows.indexesLen > 0
}

// 读取行数据
func (p *taskScheduler) readRows(file *File) error {
	// 获取工作表
	if p.sheetName = file.GetSheetName(0); p.sheetName == "" {
		p.outputError("sheet is empty. ")
		return errors.New("sheet is empty. ")
	}

	var err error
	p.rows, err = file.GetRows(p.sheetName)
	if err != nil {
		p.outputError("GetRows error: %s", err.Error())
		return err
	}

	p.rowsCount = len(p.rows)
	return nil
}

// 设置唯一列映射行数量
func (p *taskScheduler) setUniqueColumnMapRowNum() error {
	if !p.isEnableGroupRows() {
		return nil
	}

	keyNum := make(map[string]int)             // map[key]num
	keyStartIndex := make(map[string]struct{}) // map[key]row index
	ok := false
	groupRowIndex := 0
	for i := p.skipRowNum; i < p.rowsCount; i++ {
		keys := make([]string, p.groupRows.indexesLen)
		for j := 0; j < p.groupRows.indexesLen; j++ {
			if p.rows == nil || p.rows[i] == nil {
				continue
			}
			keys[j] = strings.TrimSpace(p.rows[i][j])
		}
		key := strings.Join(keys, ":")
		keyNum[key]++

		p.groupRows.rows = append(p.groupRows.rows, nil) // 默认给nil, 只有被需要被找寻的row才会被初始化数据
		_, ok = keyStartIndex[key]
		if ok {
			continue
		}
		keyStartIndex[key] = struct{}{}
		groupRowIndex = i - p.skipRowNum
		p.groupRows.keyMapIndex[key] = groupRowIndex
		p.groupRows.rows[groupRowIndex] = &uniqueColumn{
			rowsData: newRows(),
			mergeNum: 0,
		}
	}

	var num int
	for key, index := range p.groupRows.keyMapIndex {
		if num, ok = keyNum[key]; ok {
			p.groupRows.rows[index].mergeNum = num
		}
	}

	return nil
}

// 获取唯一列的key
func (p *taskScheduler) getUniqueColumnIndex(rowData []string) (index int, ok bool) {
	rowDataCount := len(rowData)
	if !p.isEnableGroupRows() {
		return 0, false
	}

	// 根据索引分组的数量为1的话只需要直接拿索引从数据中返回
	if p.groupRows.indexesLen == 1 {
		// 避免越界，索引超过行数据数量可能不是一个
		if p.groupRows.indexes[0] >= rowDataCount {
			return 0, false
		}
		if index, ok = p.groupRows.keyMapIndex[rowData[p.groupRows.indexes[0]]]; ok {
			return index, true
		}
		return 0, false
	}

	keys := make([]string, p.groupRows.indexesLen)
	for i := 0; i < p.groupRows.indexesLen; i++ {
		if p.groupRows.indexes[i] >= rowDataCount {
			return 0, false
		}
		keys[i] = strings.TrimSpace(rowData[p.groupRows.indexes[i]])
	}
	if index, ok = p.groupRows.keyMapIndex[strings.Join(keys, ":")]; ok {
		return index, true
	}
	return 0, false
}

func (p *taskScheduler) SetUniqueColumn(idxes ...int) error {
	if len(idxes) == 0 {
		return nil
	}
	p.setTransferStruct()

	sort.Ints(idxes)  // 排序，从小到大，便于列计数时使用列迭代器的指针下移次数
	var indexes []int // 去重后的 index，避免取相同列数据作为 key
	for i := 0; i < len(idxes); i++ {
		if i > 0 && idxes[i] == idxes[i-1] {
			continue
		}
		if idxes[i] < 0 {
			return errors.New("set unique column error: exceeds the min number of TransferStruct. ")
		}
		if idxes[i] >= p.transferStruct.fieldNum {
			return errors.New("set unique column error: exceeds the max number of TransferStruct. ")
		}
		indexes = append(indexes, idxes[i])
	}

	p.groupRows.indexes = indexes
	p.groupRows.indexesLen = len(p.groupRows.indexes)

	return nil
}

func (p *taskScheduler) SetErrorWriteBackMode(mode ErrorWriteBackMode) {
	if mode != ErrorWriteBackModeAnyRow && mode != ErrorWriteBackModeAssignRow {
		return
	}

	p.groupRows.errorWriteBackMode = mode
}

func (p *taskScheduler) setTransferStruct() {
	p.transferStruct.typeOf = reflect.TypeOf(p.implementor.TransferStruct())
	if p.transferStruct.typeOf.Kind() == reflect.Ptr {
		p.transferStruct.typeOf = p.transferStruct.typeOf.Elem()
	}

	p.transferStruct.fieldNum = p.transferStruct.typeOf.NumField() // 转换结构体字段数量
	p.maxColumnNum = p.transferStruct.fieldNum                     // 初始化最大列数
}

func (p *taskScheduler) parseRowData(rowData []string) (iRowData interface{}) {
	iRowData = p.implementor.TransferStruct()
	va := reflect.ValueOf(iRowData)
	if va.Kind() == reflect.Ptr {
		va = va.Elem()
	}

	// 转换结构体字段数量
	rowDataCount := len(rowData)

	// 补全空列
	if rowDataCount < p.transferStruct.fieldNum {
		rowData = append(rowData, make([]string, p.transferStruct.fieldNum-rowDataCount)...)
		rowDataCount = p.transferStruct.fieldNum
	}

	for i := 0; i < rowDataCount; i++ {
		// 超过了定义的结构体列数, 后面的列数将不再做任何解析. 直接返回该行的数据
		if i+1 > p.transferStruct.fieldNum {
			return iRowData
		}
		//style, _ := xlsx.NewStyle(`{"number_format": 21}`)
		//xlsx.SetCellStyle("Sheet1", "B2", "B2", style)
		data := strings.TrimSpace(rowData[i])
		switch p.transferStruct.typeOf.Field(i).Type.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			n64Data, _ := strconv.ParseInt(data, 10, 64)
			va.Field(i).SetInt(n64Data)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			n64Data, _ := strconv.ParseUint(data, 10, 64)
			va.Field(i).SetUint(n64Data)
		case reflect.Float32, reflect.Float64:
			f64Data, _ := strconv.ParseFloat(data, 64)
			va.Field(i).SetFloat(f64Data)
		case reflect.String:
			va.Field(i).SetString(data)
		case reflect.Struct: // 可能是: 附加列
			switch p.transferStruct.typeOf.Field(i).Type {
			case extraColumnType: // 附加列

				// 附加列的时候。需要将最大列数往后移动。这里涉及 error 列的数据写入
				if rowDataCount > p.maxColumnNum {
					p.maxColumnNum = rowDataCount
				}

				pData := ExtraColumn{}
				for j := i; j < p.headerFirstData.num; j++ {
					// 避免存在空列，但实际上是模板问题（中间的空列也将被忽略）
					if strings.TrimSpace(p.headerFirstData.data[j]) == "" {
						continue
					}
					pData.headerData = append(pData.headerData, p.headerFirstData.data[j])

					// 附加列数据获取
					var extraColumnData string
					if j <= len(rowData)-1 {
						extraColumnData = rowData[j]
					}

					pData.data = append(pData.data, strings.TrimSpace(extraColumnData))
				}
				va.Field(i).Set(reflect.ValueOf(pData))
			}
		}
	}
	return iRowData
}
