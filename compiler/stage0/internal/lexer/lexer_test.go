package lexer

import "testing"

func TestCharLiterals(t *testing.T) {
	src := "'a' '\\n' '\\\\' '\\''"
	l := New("test.dast", src)

	var chars []string
	for {
		tok := l.Next()
		if tok.Kind == TokenEOF {
			break
		}
		if tok.Kind != TokenChar {
			t.Fatalf("expected TokenChar, got %s (%q)", tok.Kind.String(), tok.Lexeme)
		}
		chars = append(chars, tok.Lexeme)
	}
	if len(chars) != 4 {
		t.Fatalf("expected 4 char tokens, got %d", len(chars))
	}
	if chars[0] != "a" {
		t.Fatalf("expected 'a', got %q", chars[0])
	}
	if chars[1] != "\n" {
		t.Fatalf("expected '\\n', got %q", chars[1])
	}
	if chars[2] != "\\" {
		t.Fatalf("expected '\\\\', got %q", chars[2])
	}
	if chars[3] != "'" {
		t.Fatalf("expected '\\'', got %q", chars[3])
	}
}
