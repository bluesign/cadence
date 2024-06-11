// Code generated by "stringer -type=PrimitiveAccess"; DO NOT EDIT.

package ast

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[AccessNotSpecified-0]
	_ = x[AccessNone-1]
	_ = x[AccessSelf-2]
	_ = x[AccessContract-3]
	_ = x[AccessAccount-4]
	_ = x[AccessAll-5]
	_ = x[AccessPubSettableLegacy-6]
}

const _PrimitiveAccess_name = "AccessNotSpecifiedAccessNoneAccessSelfAccessContractAccessAccountAccessAllAccessPubSettableLegacy"

var _PrimitiveAccess_index = [...]uint8{0, 18, 28, 38, 52, 65, 74, 97}

func (i PrimitiveAccess) String() string {
	if i >= PrimitiveAccess(len(_PrimitiveAccess_index)-1) {
		return "PrimitiveAccess(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _PrimitiveAccess_name[_PrimitiveAccess_index[i]:_PrimitiveAccess_index[i+1]]
}
