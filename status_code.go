package rod_helper

type StatusCodeInfo struct {
	Codes          []int     // 状态码
	Operator       Operator  // 操作符
	WillDo         PageCheck // 检测到这些状态码准备干什么
	NeedPunishment bool      // 是否需要惩罚，也就是这个节点会被加以一段时间的封禁不使用
}

type Operator int

const (
	Match     Operator = iota + 1 // 等于的情况，就触发
	GreatThan                     // 大于
	LessThan                      // 小于
)

func (s Operator) String() string {
	switch s {
	case Match:
		return "=="
	case GreatThan:
		return ">"
	case LessThan:
		return "<"
	default:
		return "Unknown"
	}
}
