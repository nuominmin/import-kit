package core

import (
	"strings"
)

// 头部规则检查器接口
type IHeaderRuleValidate interface {
	Validate(firstTplRow, firstUploadRow []string) bool
}

// 头部规则检查器
type HeaderRuleValidate struct {
	FixedColumnCount  int               // 固定列数
	AllowedStartValue []string          // 允许在固定列后开始的 Rule.Value
	AllowedEndValue   []string          // 允许结束的 Rule.Value。未赋值取 RuleGroups 最后一个 Value
	RuleGroups        []HeaderRuleGroup // 规则组
}

// 一个新的头部规则检查器
func NewHeaderRuleValidate(fixedColumnCount int, allowedStartValue []string, ruleGroups ...HeaderRuleGroup) *HeaderRuleValidate {
	// 可能存在下标越界，规则长度初始化为 0
	ruleGroupsIndex := len(ruleGroups) - 1
	ruleGroupsValueIndex := len(ruleGroups[ruleGroupsIndex].Values) - 1
	ruleGroupLastValue := []string{
		ruleGroups[ruleGroupsIndex].Values[ruleGroupsValueIndex].Value,
	}

	return &HeaderRuleValidate{
		FixedColumnCount:  fixedColumnCount,
		AllowedStartValue: allowedStartValue,
		AllowedEndValue:   ruleGroupLastValue,
		RuleGroups:        ruleGroups,
	}
}

// 一个新的规则允许开始值
func NewHeaderRuleAllowedStartValue(values ...string) []string {
	return values
}

// 一个新的不重复规则组
func NewNoRepetitionHeaderRuleGroup(rules ...HeaderRule) HeaderRuleGroup {
	return HeaderRuleGroup{
		Values:       rules,
		IsRepetition: false,
	}
}

// 一个新的重复规则组
func NewRepetitionHeaderRuleGroup(rules ...HeaderRule) HeaderRuleGroup {
	return HeaderRuleGroup{
		Values:       rules,
		IsRepetition: true,
	}
}

// 一个新的不重复规则
func NewNoRepetitionHeaderRule(value string) HeaderRule {
	return HeaderRule{
		Value:        value,
		IsRepetition: false,
	}
}

// 一个新的重复规则
func NewRepetitionHeaderRule(value string) HeaderRule {
	return HeaderRule{
		Value:        value,
		IsRepetition: true,
	}
}

// 检查器
func (p HeaderRuleValidate) Validate(firstTplRow, firstUploadRow []string) bool {
	// 只校验非空的列
	temp := []string{}
	for i := 0; i < len(firstUploadRow); i++ {
		row := strings.TrimSpace(firstUploadRow[i])
		if row != "" {
			temp = append(temp, row)
		}
	}
	firstUploadRow = temp

	firstUploadRowLength := len(firstUploadRow) // 上传文件列长度
	firstTplRowLength := len(firstTplRow)       // 模板文件列长度
	fixedColumnCount := p.FixedColumnCount      // 固定列数

	// 上传文件列数不能小于固定列数、模板列数不能小于固定列数、上传文件列数不能小于模板列数、上传文件列数不能等于0。 可能存在是固定列数量设置问题。返回 err？
	if firstUploadRowLength < fixedColumnCount || firstTplRowLength < fixedColumnCount || firstUploadRowLength < firstTplRowLength || firstUploadRowLength == 0 {
		return false
	}

	if firstUploadRowLength == firstTplRowLength {
		return equalStringSlice(firstTplRow, firstUploadRow) // 和模板长度相同，提交的文件不存在附加列数据，只需要校验固定字段，不需要检查规则，判断后直接返回结果
	}

	return p.validate(firstTplRow, firstUploadRow)
}

// 检查固定列和模板的头部是否一致、检查固定列后的值是否允许
func (p HeaderRuleValidate) validateFixed(firstTplRow, firstUploadRow []string) bool {
	var (
		fixedColumnCount  = p.FixedColumnCount  // 固定列数
		firstTplRowLength = len(firstTplRow)    // 模板文件列长度
		allowedStartValue = p.AllowedStartValue // 允许在固定列数后开始的值

		existAtAllowedStartValue = func(value string) bool { // 存在于允许的起始值
			for i := 0; i < len(allowedStartValue); i++ {
				if allowedStartValue[i] == value {
					return true
				}
			}
			return false
		}
	)

	if fixedColumnCount == 0 {
		if firstTplRowLength != 0 {
			return false
		}
		if !existAtAllowedStartValue(firstUploadRow[0]) {
			return false // 全部都是附加列，不需要检查固定列和模板的行数据是否匹配，但需要找寻第一列的头部是否在允许起始值里面和匹配规则
		}
	} else {
		// 校验固定字段
		for i := 0; i < fixedColumnCount; i++ {
			if firstUploadRow[i] != firstTplRow[i] {
				return false
			}
		}
	}
	return true
}

// 头部规则组
type HeaderRuleGroup struct {
	Values       []HeaderRule // 标识值, 存在重复多列为一组的情况
	IsRepetition bool         // 是否重复列，不重复：[a,b,c,d,a,b,c,d]，重复：[a,b,c,d,c,d]、[a,b,c,c,d,a,b,c,d]
}

// 头部规则
type HeaderRule struct {
	Value        string // 标识值
	IsRepetition bool   // 是否重复列，不重复：[a,b,a,b]，重复：[a,b,b,a,b,a,b]、[a,a,b,a,b]、[a,b,a,b,b]
}

// 头部规则节点
type headerRuleNode struct {
	value     string // 标识值
	nextIndex []int  // 下一个节点的索引
}

/*
RuleGroups: []model.HeaderRuleGroup{
	{Values: []model.HeaderRule{
		{Value: a},
		{Value: b},
	}, IsRepetition: false},
	{Values: []model.HeaderRule{
		{Value: c},
		{Value: d},
	}, IsRepetition: true},
},

gen nodes result:[
	0:{"value":"a","next_index":[1]},
	1:{"value":"b","next_index":[2]},
	2:{"value":"c","next_index":[3]},
	4:{"value":"d","next_index":[0,3]},
]
*/

// 规则转为节点
func (p HeaderRuleValidate) nodes() []*headerRuleNode {
	mapNodeIndex := make(map[string]int)      // 节点索引
	mapNextNodes := make(map[string][]string) // 下一个节点
	ruleGroupsLength := len(p.RuleGroups)
	res := []*headerRuleNode{}
	for i := 0; i < ruleGroupsLength; i++ {
		valuesLength := len(p.RuleGroups[i].Values)
		for j := 0; j < valuesLength; j++ {
			value := p.RuleGroups[i].Values[j].Value
			mapNodeIndex[value] = len(res)
			res = append(res, &headerRuleNode{
				value:     value,
				nextIndex: []int{},
			})
			if p.RuleGroups[i].Values[j].IsRepetition {
				mapNextNodes[value] = append(mapNextNodes[value], value)
			}
			if j == valuesLength-1 {
				if p.RuleGroups[i].IsRepetition {
					mapNextNodes[value] = append(mapNextNodes[value], p.RuleGroups[i].Values[0].Value)
				}
				if ruleGroupsLength > 1 {
					mapNextNodes[value] = append(mapNextNodes[value], p.RuleGroups[(i+1)%ruleGroupsLength].Values[0].Value)
				}
			} else {
				mapNextNodes[value] = append(mapNextNodes[value], p.RuleGroups[i].Values[j+1].Value)
			}
		}
	}
	for i := 0; i < len(res); i++ {
		nextNodes := mapNextNodes[res[i].value]
		nextNodesLength := len(nextNodes)
		res[i].nextIndex = make([]int, nextNodesLength)
		for j := 0; j < nextNodesLength; j++ {
			res[i].nextIndex[j] = mapNodeIndex[nextNodes[j]]
		}
	}
	return res
}

// 检查器
func (p HeaderRuleValidate) validate(firstTplRow, firstUploadRow []string) bool {

	// 检查固定头部
	// 检查最后一个值是否被允许
	// 根据规则检查附加头部

	//if !p.validateFixed(firstTplRow, firstUploadRow) {
	//	fmt.Println(1)
	//	return false
	//}
	//
	//if !p.validateAllowedEndValue(firstUploadRow) {
	//	fmt.Println(2)
	//	return false
	//}
	//
	//if !p.validateRule(firstUploadRow) {
	//	fmt.Println(3)
	//	return false
	//}

	return p.validateFixed(firstTplRow, firstUploadRow) && p.validateAllowedEndValue(firstUploadRow) && p.validateRule(firstUploadRow)
}

// 检查最后一个值是否被允许
func (p HeaderRuleValidate) validateAllowedEndValue(firstUploadRow []string) bool {
	lastUploadHeaderRow := firstUploadRow[len(firstUploadRow)-1]
	for i := 0; i < len(p.AllowedEndValue); i++ {
		if p.AllowedEndValue[i] == lastUploadHeaderRow {
			return true
		}
	}
	return false
}

// 检查上传文件头部列数据和规则是否匹配
func (p HeaderRuleValidate) validateRule(firstUploadRow []string) bool {
	var (
		nodes                = p.nodes()           // 节点列表
		nodesLength          = len(nodes)          // 节点列表长度
		currentNodeIndex     = 0                   // 当前节点索引
		fixedColumnCount     = p.FixedColumnCount  // 固定列数
		firstUploadRowLength = len(firstUploadRow) // 上传文件列长度
		startIndex           = fixedColumnCount    // 起始指针位置
	)

	// 先确定从哪个节点开始
	for i := 0; i < nodesLength; i++ {
		if firstUploadRow[startIndex] == nodes[i].value {
			currentNodeIndex = i
			startIndex++
			break
		}
	}

	// 没有找到
	if startIndex == fixedColumnCount {
		return false
	}

	for ; startIndex < firstUploadRowLength; startIndex++ {
		isFind := false
		for i := 0; i < len(nodes[currentNodeIndex].nextIndex); i++ {
			if nodes[nodes[currentNodeIndex].nextIndex[i]].value == firstUploadRow[startIndex] {
				currentNodeIndex = nodes[currentNodeIndex].nextIndex[i]
				isFind = true
				break
			}
		}
		if !isFind {
			return false
		}
	}

	return true
}
