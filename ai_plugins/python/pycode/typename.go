package pycode

type TypeName interface {
	String() string
	Equals(other TypeName) bool
	IsTypeName()
}

var basics = make(map[string]struct{})
