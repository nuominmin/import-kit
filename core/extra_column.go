package core

import (
	"reflect"
)

var (
	extraColumnType = reflect.TypeOf(ExtraColumn{})
)

// 附加列
type ExtraColumn struct {
	headerData []string // 多余的列数据对应的表头
	data       []string // 多余的列数据
}

func (p *ExtraColumn) GetHeaderData() []string {
	return p.headerData
}

func (p *ExtraColumn) GetData() []string {
	return p.data
}
