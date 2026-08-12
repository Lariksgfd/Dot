package lexer

import "fmt"

// TokenType identifies the lexical class of a token.
type TokenType int

// Token types, declared in the group order documented in PROGRESS.md.
// The unexported sentinels delimit the groups so that the classification
// helpers can be simple range checks.
const (
	// --- Group 0: special ---

	// TokenIllegal is an invalid character or unrecoverable lexeme.
	TokenIllegal TokenType = iota
	// TokenEOF marks the end of the source.
	TokenEOF
	// TokenNewline is a statement terminator (Dot has no semicolons).
	TokenNewline

	// --- Group 1: literals and identifiers ---

	// TokenIdent is an identifier such as foo, _helper or Point.
	TokenIdent
	// TokenUnderscore is a lone "_" (wildcard / discard pattern).
	TokenUnderscore
	// TokenInt is an integer literal in any supported base.
	TokenInt
	// TokenFloat is a floating point literal.
	TokenFloat
	// TokenString is a double-quoted string literal, possibly interpolated.
	TokenString
	// TokenRawString is a backtick string literal: no escapes, no interpolation.
	TokenRawString

	tokenKeywordStart

	// --- Group 2: keywords ---

	// TokenFn is the keyword "fn".
	TokenFn
	// TokenStruct is the keyword "struct".
	TokenStruct
	// TokenTrait is the keyword "trait".
	TokenTrait
	// TokenImpl is the keyword "impl".
	TokenImpl
	// TokenEnum is the keyword "enum".
	TokenEnum
	// TokenIf is the keyword "if".
	TokenIf
	// TokenElse is the keyword "else".
	TokenElse
	// TokenFor is the keyword "for".
	TokenFor
	// TokenIn is the keyword "in".
	TokenIn
	// TokenMatch is the keyword "match".
	TokenMatch
	// TokenReturn is the keyword "return".
	TokenReturn
	// TokenBreak is the keyword "break".
	TokenBreak
	// TokenContinue is the keyword "continue".
	TokenContinue
	// TokenImport is the keyword "import".
	TokenImport
	// TokenFrom is the keyword "from".
	TokenFrom
	// TokenConst is the keyword "const".
	TokenConst
	// TokenMut is the keyword "mut".
	TokenMut
	// TokenSelf is the keyword "self" (the receiver value).
	TokenSelf
	// TokenSelfType is the keyword "Self" (the receiver type).
	TokenSelfType
	// TokenSpawn is the keyword "spawn".
	TokenSpawn
	// TokenAsync is the keyword "async".
	TokenAsync
	// TokenAwait is the keyword "await".
	TokenAwait
	// TokenDefer is the keyword "defer".
	TokenDefer
	// TokenEmbed is the keyword "embed".
	TokenEmbed
	// TokenPub is the keyword "pub".
	TokenPub
	// TokenTrue is the boolean literal keyword "true".
	TokenTrue
	// TokenFalse is the boolean literal keyword "false".
	TokenFalse
	// TokenNil is the keyword "nil".
	TokenNil
	// TokenAnd is the logical operator keyword "and".
	TokenAnd
	// TokenOr is the logical operator keyword "or".
	TokenOr
	// TokenNot is the logical operator keyword "not".
	TokenNot
	// TokenAs is the keyword "as" (cast / import alias).
	TokenAs
	// TokenIs is the keyword "is" (type test).
	TokenIs
	// TokenDyn is the keyword "dyn" (dynamic trait object).
	TokenDyn
	// TokenWeak is the keyword "weak" (weak reference).
	TokenWeak

	tokenKeywordEnd
	tokenReservedStart

	// --- Group 3: reserved for future versions ---

	// TokenMacro is the reserved word "macro".
	TokenMacro
	// TokenWhere is the reserved word "where".
	TokenWhere
	// TokenYield is the reserved word "yield".
	TokenYield
	// TokenTypeKw is the reserved word "type".
	TokenTypeKw
	// TokenModule is the reserved word "module".
	TokenModule
	// TokenUnsafe is the reserved word "unsafe".
	TokenUnsafe
	// TokenExtern is the reserved word "extern".
	TokenExtern
	// TokenStatic is the reserved word "static".
	TokenStatic
	// TokenVirtual is the reserved word "virtual".
	TokenVirtual
	// TokenOverride is the reserved word "override".
	TokenOverride

	tokenReservedEnd
	tokenOperatorStart

	// --- Group 4: arithmetic operators ---

	// TokenPlus is "+".
	TokenPlus
	// TokenMinus is "-".
	TokenMinus
	// TokenStar is "*".
	TokenStar
	// TokenSlash is "/".
	TokenSlash
	// TokenPercent is "%".
	TokenPercent
	// TokenStarStar is "**".
	TokenStarStar

	// --- Group 5: comparison operators ---

	// TokenEqEq is "==".
	TokenEqEq
	// TokenBangEq is "!=".
	TokenBangEq
	// TokenLt is "<".
	TokenLt
	// TokenGt is ">".
	TokenGt
	// TokenLtEq is "<=".
	TokenLtEq
	// TokenGtEq is ">=".
	TokenGtEq

	// --- Group 6: bitwise operators ---

	// TokenAmp is "&".
	TokenAmp
	// TokenPipe is "|" (also the match-arm alternative separator).
	TokenPipe
	// TokenCaret is "^".
	TokenCaret
	// TokenTilde is "~".
	TokenTilde
	// TokenShl is "<<".
	TokenShl
	// TokenShr is ">>".
	TokenShr

	// --- Group 7: assignment operators ---

	// TokenAssign is "=".
	TokenAssign
	// TokenPlusAssign is "+=".
	TokenPlusAssign
	// TokenMinusAssign is "-=".
	TokenMinusAssign
	// TokenStarAssign is "*=".
	TokenStarAssign
	// TokenSlashAssign is "/=".
	TokenSlashAssign
	// TokenPercentAssign is "%=".
	TokenPercentAssign
	// TokenAmpAssign is "&=".
	TokenAmpAssign
	// TokenPipeAssign is "|=".
	TokenPipeAssign
	// TokenCaretAssign is "^=".
	TokenCaretAssign
	// TokenShlAssign is "<<=".
	TokenShlAssign
	// TokenShrAssign is ">>=".
	TokenShrAssign

	// --- Group 8: special operators ---

	// TokenPipeArrow is "|>".
	TokenPipeArrow
	// TokenDotDot is "..".
	TokenDotDot
	// TokenDotDotEq is "..=".
	TokenDotDotEq
	// TokenEllipsis is "...".
	TokenEllipsis
	// TokenArrow is "->".
	TokenArrow
	// TokenFatArrow is "=>".
	TokenFatArrow
	// TokenQuestion is "?".
	TokenQuestion

	tokenOperatorEnd

	// --- Group 9: punctuation and delimiters ---

	// TokenDot is ".".
	TokenDot
	// TokenComma is ",".
	TokenComma
	// TokenColon is ":".
	TokenColon
	// TokenAt is "@".
	TokenAt
	// TokenLParen is "(".
	TokenLParen
	// TokenRParen is ")".
	TokenRParen
	// TokenLBracket is "[".
	TokenLBracket
	// TokenRBracket is "]".
	TokenRBracket
	// TokenLBrace is "{".
	TokenLBrace
	// TokenRBrace is "}".
	TokenRBrace

	// --- Group 10: comments ---

	// TokenComment is a line or block comment.
	TokenComment
)

// String returns the canonical name of the token type, e.g. "TokenFn", used
// in error messages and tests.
func (t TokenType) String() string {
	if name, ok := tokenNames[t]; ok {
		return name
	}
	return fmt.Sprintf("TokenType(%d)", int(t))
}

// Literal returns the fixed source spelling for operators, punctuation and
// keywords (e.g. "->", "fn"). It returns "" for tokens whose text varies
// (TokenIdent, TokenInt, TokenFloat, TokenString, TokenRawString, TokenEOF...).
func (t TokenType) Literal() string {
	return tokenSpellings[t]
}

// IsKeyword reports whether t is one of the language keywords.
func (t TokenType) IsKeyword() bool {
	return t > tokenKeywordStart && t < tokenKeywordEnd
}

// IsReserved reports whether t is a reserved-for-future keyword.
func (t TokenType) IsReserved() bool {
	return t > tokenReservedStart && t < tokenReservedEnd
}

// IsOperator reports whether t is an operator. The keyword operators
// "and", "or" and "not" are included.
func (t TokenType) IsOperator() bool {
	if t > tokenOperatorStart && t < tokenOperatorEnd {
		return true
	}
	switch t {
	case TokenAnd, TokenOr, TokenNot:
		return true
	}
	return false
}

// IsLiteral reports whether t is a literal token (int, float, string, raw
// string, true, false, nil).
func (t TokenType) IsLiteral() bool {
	switch t {
	case TokenInt, TokenFloat, TokenString, TokenRawString,
		TokenTrue, TokenFalse, TokenNil:
		return true
	}
	return false
}

// LookupKeyword maps an identifier spelling to its keyword token type.
// It returns (TokenIdent, false) when s is not a keyword or reserved word.
func LookupKeyword(s string) (TokenType, bool) {
	if t, ok := keywords[s]; ok {
		return t, true
	}
	return TokenIdent, false
}
