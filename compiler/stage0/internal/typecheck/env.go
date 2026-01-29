package typecheck

type env struct {
	scopes []map[string]VarInfo
}

func newEnv() *env {
	return &env{}
}

func (e *env) push() {
	e.scopes = append(e.scopes, map[string]VarInfo{})
}

func (e *env) pop() {
	if len(e.scopes) == 0 {
		return
	}
	e.scopes = e.scopes[:len(e.scopes)-1]
}

func (e *env) declare(name string, info VarInfo) {
	if len(e.scopes) == 0 {
		e.push()
	}
	e.scopes[len(e.scopes)-1][name] = info
}

func (e *env) lookup(name string) (VarInfo, bool) {
	for i := len(e.scopes) - 1; i >= 0; i-- {
		if v, ok := e.scopes[i][name]; ok {
			return v, true
		}
	}
	return VarInfo{Type: Type{Kind: TypeInvalid}}, false
}
