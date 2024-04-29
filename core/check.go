package core

import (
	"errors"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

type ICheckService interface {
	Run() (totalRows int64, err error)
	SetHeaderRule(*HeaderRuleValidate) ICheckService
}

type checkService struct {
	openTplFileFunc, openImportFileFUnc OpenFileFunc
	skipRowNum                          int
	maxRowNum                           int64
	headerRuleValidate                  IHeaderRuleValidate

	tplFile, importFile         *File
	tplFileRows, importFileRows *excelize.Rows
}

func NewCheckService(openTplFileFunc, openImportFileFUnc OpenFileFunc, skipRowNum int, maxRowNum int64) ICheckService {
	return &checkService{
		openTplFileFunc:    openTplFileFunc,
		openImportFileFUnc: openImportFileFUnc,
		skipRowNum:         skipRowNum,
		maxRowNum:          maxRowNum,
	}
}

type ErrorMessage string

const (
	MaxRowNumError        ErrorMessage = "max row num error. "                       // 最大行数错误
	TemplateError         ErrorMessage = "The template is incorrect. "               // 模板错误
	LargestRowNumberError ErrorMessage = "The maximum number of rows was exceeded. " // 超过最大行数
	EmptyFile             ErrorMessage = "This is an empty file. "                   // 空文件

	unknownError ErrorMessage = "unknown error: %s" // 未知错误
)

func (p ErrorMessage) Error() error {
	return errors.New(string(p))
}

func (p ErrorMessage) Sprintf(e ...error) error {
	a := []interface{}{}
	for i := 0; i < len(e); i++ {
		if e[i] == nil {
			continue
		}
		a = append(a, e[i].Error())
	}
	return errors.New(fmt.Sprintf(string(p), a...))
}

func (p *checkService) SetHeaderRule(headerRuleValidate *HeaderRuleValidate) ICheckService {
	p.headerRuleValidate = headerRuleValidate
	return p
}

func (p *checkService) Run() (totalRows int64, err error) {
	p.tplFile, err = p.openTplFileFunc()
	if err != nil {
		return 0, fmt.Errorf("open tpl file error: %+v", err)
	}
	defer func() {
		if err1 := p.tplFile.Close(); err1 != nil {
			fmt.Printf("close tpl file error: %+v \n", err1)
		}
	}()

	p.importFile, err = p.openImportFileFUnc()
	if err != nil {
		return 0, fmt.Errorf("open import file error: %+v", err)
	}
	defer func() {
		if err1 := p.importFile.Close(); err1 != nil {
			fmt.Printf("close import file error: %+v \n", err1)
		}
	}()
	if p.maxRowNum <= 0 {
		return 0, MaxRowNumError.Error()
	}

	if p.importFile == nil || p.tplFile == nil {
		return 0, TemplateError.Error()
	}

	p.tplFileRows, err = p.tplFile.Rows(p.tplFile.GetSheetName(0))
	if err != nil {
		return 0, unknownError.Sprintf(err)
	}
	defer func() {
		if err1 := p.tplFileRows.Close(); err1 != nil {
			fmt.Printf("close tpl file rows error: %+v \n", err1)
		}
	}()

	p.importFileRows, err = p.importFile.Rows(p.importFile.GetSheetName(0))
	if err != nil {
		return 0, unknownError.Sprintf(err)
	}
	defer func() {
		if err1 := p.importFileRows.Close(); err1 != nil {
			fmt.Printf("close import file rows error: %+v \n", err1)
		}
	}()

	// 头部校验，包含附加列的头部校验
	if !p.compareHeader() {
		return 0, TemplateError.Error()
	}

	totalRows = p.getTotalRows()

	if totalRows <= 0 {
		return 0, EmptyFile.Error()
	}

	// 总行数校验
	if totalRows > p.maxRowNum {
		return 0, LargestRowNumberError.Error()
	}

	return totalRows, nil
}

func (p *checkService) compareHeader() bool {

	// 初始化了头部规则校验器，只需要检查固定列是否和模板一致和检查附加列的符合是否规则，然后直接返回结果
	if p.headerRuleValidate != nil {
		p.importFileRows.Next()
		importFileRow, _ := p.importFileRows.Columns()
		p.tplFileRows.Next()
		tplFileRow, _ := p.tplFileRows.Columns()
		return p.headerRuleValidate.Validate(tplFileRow, importFileRow)
	}

	for i := 0; i < p.skipRowNum; i++ {
		p.importFileRows.Next()
		importFileRow, _ := p.importFileRows.Columns()
		p.tplFileRows.Next()
		tplFileRow, _ := p.tplFileRows.Columns()

		// 头部校验
		if !equalStringSlice(tplFileRow, importFileRow) {
			return false
		}
	}
	return true
}

// 获取总行数
func (p *checkService) getTotalRows() (total int64) {
	for p.importFileRows.Next() {
		rowData, _ := p.importFileRows.Columns()
		if isEmpty(rowData) {
			continue
		}
		total++
	}
	return total
}

// 对比两个 []stirng 是否相同
func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

// 是否为空
func isEmpty(ss []string) bool {
	for i := 0; i < len(ss); i++ {
		if strings.TrimSpace(ss[i]) != "" {
			return false
		}
	}
	return true
}
