package source

type Position struct {
	Filename string
	Line     int
	Column   int
}

type Span struct {
	Start Position
	End   Position
}

func ZeroSpan() Span {
	return Span{}
}
