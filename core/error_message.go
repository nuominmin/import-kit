package core

import (
	"fmt"
	"sort"

	"github.com/xuri/excelize/v2"
)

type IErrorMessages interface {
	// GetErrFile 获取错误文件
	GetErrFile() *File

	// Count 错误数量
	Count() int

	// Append 追加错误
	Append(rowIndex int, err error)

	// Build 打包错误文件
	Build(rows [][]string, maxColumnNum, skipRowNum int) (err error)
}

type errorMessages struct {
	errors     []*errorMessage
	errFile    *File
	errStyleID int
}
type errorMessage struct {
	rowIndex int
	err      error
}

func newErrorMessages() IErrorMessages {
	return &errorMessages{}
}

func (p *errorMessages) newErrFile() (streamWriter *excelize.StreamWriter, err error) {
	if p.errFile == nil {
		p.errFile = newFile()
	}

	if streamWriter, err = p.errFile.NewStreamWriter("Sheet1"); err != nil {
		return nil, fmt.Errorf("new stream write error: %s", err.Error())
	}

	p.errStyleID, err = p.errFile.NewStyle(&excelize.Style{Font: &excelize.Font{Color: "FF0000"}})
	if err != nil {
		return nil, fmt.Errorf("NewStyle error: %s", err.Error())
	}

	return streamWriter, nil
}

func (p *errorMessages) GetErrFile() *File {
	return p.errFile
}

func (p *errorMessages) Append(rowIndex int, err error) {
	p.errors = append(p.errors, &errorMessage{
		rowIndex: rowIndex,
		err:      err,
	})
}

func (p *errorMessages) Count() int {
	return len(p.errors)
}

// 组装数据
func (p *errorMessages) assembleData(d ...string) []interface{} {
	if len(d) == 0 {
		return []interface{}{}
	}
	lastValue := len(d) - 1
	res := make([]interface{}, len(d))
	for i := 0; i < lastValue; i++ {
		res[i] = excelize.Cell{Value: d[i]}
	}

	// 最后一列的数据写入和样式写入
	res[lastValue] = excelize.Cell{StyleID: p.errStyleID, Value: d[lastValue]}
	return res
}

func (p *errorMessages) Build(rows [][]string, maxColumnNum, skipRowNum int) (err error) {
	if p.Count() == 0 {
		return nil
	}

	sort.Slice(p.errors, func(i, j int) bool {
		return p.errors[i].rowIndex < p.errors[j].rowIndex
	})

	// 写入错误消息到每一行
	mapErrorRowIndex := make(map[int]int) // map[row index] error index
	for i := 0; i < p.Count(); i++ {
		//if p.errors[i].err == nil {
		//	log.Error("[importkit] errorMessages error is nil. ")
		//	continue
		//}

		mapErrorRowIndex[p.errors[i].rowIndex] = i
	}

	var streamWriter *excelize.StreamWriter
	streamWriter, err = p.newErrFile()
	if err != nil {
		return fmt.Errorf("newErrFile error: %s", err.Error())
	}

	var ok bool
	var errIndex int

	for i := 0; i < len(rows); i++ {
		rowData := make([]string, maxColumnNum+1)
		copy(rowData, rows[i])
		rows[i] = nil

		if i < skipRowNum {
			if i == 0 {
				rowData[maxColumnNum] = "错误提示"
			}
			_ = streamWriter.SetRow(fmt.Sprintf("A%d", i+1), p.assembleData(rowData...))
			continue
		}

		errIndex, ok = mapErrorRowIndex[i]
		if !ok {
			continue
		}

		if p.errors[errIndex].err != nil {
			rowData[maxColumnNum] = p.errors[errIndex].err.Error()
		}

		_ = streamWriter.SetRow(fmt.Sprintf("A%d", errIndex+skipRowNum+1), p.assembleData(rowData...))
	}

	if err = streamWriter.Flush(); err != nil {
		return fmt.Errorf("stream write flush error: %s", err.Error())
	}

	p.errors = nil
	mapErrorRowIndex = nil

	return nil
}
