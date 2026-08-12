package lexer

// tokenInfo is the single source of truth for token metadata: the canonical
// Go constant name of every token type and, where the token has a fixed
// source spelling, that spelling. The lookup tables below are derived from
// it so they can never drift apart.
var tokenInfo = []struct {
	tt       TokenType
	name     string
	spelling string
}{
	{TokenIllegal, "TokenIllegal", ""},
	{TokenEOF, "TokenEOF", ""},
	{TokenNewline, "TokenNewline", ""},

	{TokenIdent, "TokenIdent", ""},
	{TokenUnderscore, "TokenUnderscore", "_"},
	{TokenInt, "TokenInt", ""},
	{TokenFloat, "TokenFloat", ""},
	{TokenString, "TokenString", ""},
	{TokenRawString, "TokenRawString", ""},

	{TokenFn, "TokenFn", "fn"},
	{TokenStruct, "TokenStruct", "struct"},
	{TokenTrait, "TokenTrait", "trait"},
	{TokenImpl, "TokenImpl", "impl"},
	{TokenEnum, "TokenEnum", "enum"},
	{TokenIf, "TokenIf", "if"},
	{TokenElse, "TokenElse", "else"},
	{TokenFor, "TokenFor", "for"},
	{TokenIn, "TokenIn", "in"},
	{TokenMatch, "TokenMatch", "match"},
	{TokenReturn, "TokenReturn", "return"},
	{TokenBreak, "TokenBreak", "break"},
	{TokenContinue, "TokenContinue", "continue"},
	{TokenImport, "TokenImport", "import"},
	{TokenFrom, "TokenFrom", "from"},
	{TokenConst, "TokenConst", "const"},
	{TokenMut, "TokenMut", "mut"},
	{TokenSelf, "TokenSelf", "self"},
	{TokenSelfType, "TokenSelfType", "Self"},
	{TokenSpawn, "TokenSpawn", "spawn"},
	{TokenAsync, "TokenAsync", "async"},
	{TokenAwait, "TokenAwait", "await"},
	{TokenDefer, "TokenDefer", "defer"},
	{TokenEmbed, "TokenEmbed", "embed"},
	{TokenPub, "TokenPub", "pub"},
	{TokenTrue, "TokenTrue", "true"},
	{TokenFalse, "TokenFalse", "false"},
	{TokenNil, "TokenNil", "nil"},
	{TokenAnd, "TokenAnd", "and"},
	{TokenOr, "TokenOr", "or"},
	{TokenNot, "TokenNot", "not"},
	{TokenAs, "TokenAs", "as"},
	{TokenIs, "TokenIs", "is"},
	{TokenDyn, "TokenDyn", "dyn"},
	{TokenWeak, "TokenWeak", "weak"},

	{TokenMacro, "TokenMacro", "macro"},
	{TokenWhere, "TokenWhere", "where"},
	{TokenYield, "TokenYield", "yield"},
	{TokenTypeKw, "TokenTypeKw", "type"},
	{TokenModule, "TokenModule", "module"},
	{TokenUnsafe, "TokenUnsafe", "unsafe"},
	{TokenExtern, "TokenExtern", "extern"},
	{TokenStatic, "TokenStatic", "static"},
	{TokenVirtual, "TokenVirtual", "virtual"},
	{TokenOverride, "TokenOverride", "override"},

	{TokenPlus, "TokenPlus", "+"},
	{TokenMinus, "TokenMinus", "-"},
	{TokenStar, "TokenStar", "*"},
	{TokenSlash, "TokenSlash", "/"},
	{TokenPercent, "TokenPercent", "%"},
	{TokenStarStar, "TokenStarStar", "**"},

	{TokenEqEq, "TokenEqEq", "=="},
	{TokenBangEq, "TokenBangEq", "!="},
	{TokenLt, "TokenLt", "<"},
	{TokenGt, "TokenGt", ">"},
	{TokenLtEq, "TokenLtEq", "<="},
	{TokenGtEq, "TokenGtEq", ">="},

	{TokenAmp, "TokenAmp", "&"},
	{TokenPipe, "TokenPipe", "|"},
	{TokenCaret, "TokenCaret", "^"},
	{TokenTilde, "TokenTilde", "~"},
	{TokenShl, "TokenShl", "<<"},
	{TokenShr, "TokenShr", ">>"},

	{TokenAssign, "TokenAssign", "="},
	{TokenPlusAssign, "TokenPlusAssign", "+="},
	{TokenMinusAssign, "TokenMinusAssign", "-="},
	{TokenStarAssign, "TokenStarAssign", "*="},
	{TokenSlashAssign, "TokenSlashAssign", "/="},
	{TokenPercentAssign, "TokenPercentAssign", "%="},
	{TokenAmpAssign, "TokenAmpAssign", "&="},
	{TokenPipeAssign, "TokenPipeAssign", "|="},
	{TokenCaretAssign, "TokenCaretAssign", "^="},
	{TokenShlAssign, "TokenShlAssign", "<<="},
	{TokenShrAssign, "TokenShrAssign", ">>="},

	{TokenPipeArrow, "TokenPipeArrow", "|>"},
	{TokenDotDot, "TokenDotDot", ".."},
	{TokenDotDotEq, "TokenDotDotEq", "..="},
	{TokenEllipsis, "TokenEllipsis", "..."},
	{TokenArrow, "TokenArrow", "->"},
	{TokenFatArrow, "TokenFatArrow", "=>"},
	{TokenQuestion, "TokenQuestion", "?"},

	{TokenDot, "TokenDot", "."},
	{TokenComma, "TokenComma", ","},
	{TokenColon, "TokenColon", ":"},
	{TokenAt, "TokenAt", "@"},
	{TokenLParen, "TokenLParen", "("},
	{TokenRParen, "TokenRParen", ")"},
	{TokenLBracket, "TokenLBracket", "["},
	{TokenRBracket, "TokenRBracket", "]"},
	{TokenLBrace, "TokenLBrace", "{"},
	{TokenRBrace, "TokenRBrace", "}"},
	{TokenComment, "TokenComment", ""},
}

var (
	// tokenNames maps a token type to its canonical Go constant name.
	tokenNames = make(map[TokenType]string, len(tokenInfo))
	// tokenSpellings maps a token type to its fixed source spelling, if any.
	tokenSpellings = make(map[TokenType]string, len(tokenInfo))
	// keywords maps a keyword or reserved-word spelling to its token type.
	keywords = make(map[string]TokenType)
)

func init() {
	for _, info := range tokenInfo {
		tokenNames[info.tt] = info.name
		if info.spelling != "" {
			tokenSpellings[info.tt] = info.spelling
		}
		if info.tt.IsKeyword() || info.tt.IsReserved() {
			keywords[info.spelling] = info.tt
		}
	}
}
