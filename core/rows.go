package core

type IRows interface {
	// Append 追加数据到 rows
	Append(data interface{}, index int)
	// Count rows 的长度
	Count() int
	// SetRowsErrs 设置所有行相同的错误
	SetRowsErrs(errs ...error)
	// Each 每一个数据
	Each(fn EachFn)
	// EachReverse 逆序循环每一个数据
	EachReverse(fn EachFn)
	// GetRow 获取行
	GetRow(i int) IRow
	// GetFirstRow 获取第一行
	GetFirstRow() IRow
	// IsErr 是否错误
	IsErr() bool
}

type IRow interface {
	// SetErrs 设置行的错误
	SetErrs(errs ...error)
	// GetErrs 获取行的错误
	GetErrs() Errors
	// GetData 获取行数据
	GetData() interface{}
	// GetFormIndex 获取整表的索引
	GetFormIndex() (formIndex int)
	// IsErr 是否错误
	IsErr() bool
}

type EachFn func(i int, row IRow) (isBreak bool)

type rows struct {
	rows    []IRow
	rowsLen int // 行数据长度， 每次 Append 都将会增加
}

type row struct {
	Data  interface{} // 行数据
	index int         // 行索引
	errs  Errors      // 错误
	isErr bool        // 是否存在错误， 设置 errs 的时候将会设置为 true
}

func newRows() IRows {
	return &rows{}
}

func (rs *rows) Each(fn EachFn) {
	for i := 0; i < rs.Count(); i++ {
		if fn(i, rs.GetRow(i)) {
			break
		}
	}
}

func (rs *rows) EachReverse(fn EachFn) {
	for i := rs.Count() - 1; i >= 0; i-- {
		if fn(i, rs.GetRow(i)) {
			break
		}
	}
}

func (rs *rows) Append(data interface{}, index int) {
	rs.rows = append(rs.rows, &row{
		Data:  data,
		index: index,
	})
	rs.rowsLen++
}

func (rs *rows) Count() int {
	return rs.rowsLen
}

func (rs *rows) GetRow(i int) IRow {
	if i >= rs.Count() {
		return &row{} // 返回一个空的，避免外部调用空指针
	}
	return rs.rows[i]
}

func (rs *rows) GetFirstRow() IRow {
	return rs.GetRow(0)
}

func (rs *rows) SetRowsErrs(errs ...error) {
	if len(errs) == 0 {
		return
	}

	for i := 0; i < rs.Count(); i++ {
		rs.GetRow(i).SetErrs(errs...)
	}
}

func (rs *rows) IsErr() bool {
	for i := 0; i < rs.Count(); i++ {
		if rs.GetRow(i).IsErr() {
			return true
		}
	}
	return false
}

// row implement

func (rs *row) SetErrs(errs ...error) {
	if len(errs) == 0 {
		return
	}
	rs.errs.Append(errs...)

	if !rs.isErr {
		rs.isErr = true
	}
}

func (rs *row) GetErrs() Errors {
	return rs.errs
}

func (rs *row) GetData() interface{} {
	return rs.Data
}
func (rs *row) GetFormIndex() (formIndex int) {
	return rs.index
}

func (rs *row) IsErr() bool {
	return rs.isErr
}
