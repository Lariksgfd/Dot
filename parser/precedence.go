package parser

import (
	"github.com/dotlang/dot/ast"
	"github.com/dotlang/dot/lexer"
)

// Binding powers. Higher binds tighter. These are SYNTAX.md's 15 levels
// inverted, plus precCast (D39) and precPostfix. See D27.
const (
	precNone    = iota // 0  not an infix operator
	precAssign         // 1  =  += -= *= /= %= &= |= ^= <<= >>=   (right)
	precPipe           // 2  |>                                   (left)
	precOr             // 3  or                                   (left)
	precAnd            // 4  and                                  (left)
	precCompare        // 5  == != < > <= >=                      (left)
	precRange          // 6  .. ..=                               (NONE)
	precBitOr          // 7  |                                    (left)
	precBitXor         // 8  ^                                    (left)
	precBitAnd         // 9  &                                    (left)
	precShift          // 10 << >>                                (left)
	precAdd            // 11 + -                                  (left)
	precMul            // 12 * / %                                (left)
	precCast           // 13 as is                                (left)
	precPower          // 14 **                                   (right)
	precUnary          // 15 prefix - not ~ + await                (right)
	precPostfix        // 16 . () [] ?                            (left)
)

// assoc describes how an operator groups when chained with itself.
type assoc int

const (
	assocLeft assoc = iota
	assocRight
	assocNone
)

// prefixFn parses a prefix (nud) expression given the parser.
type prefixFn func(p *parser) ast.Expr

// infixFn parses an infix (led) expression given the parser and the already
// parsed left operand.
type infixFn func(p *parser, left ast.Expr) ast.Expr

// prefixFns maps a token type to its prefix handler, populated by the
// expression-primary task (3.5).
var prefixFns = map[lexer.TokenType]prefixFn{}

// infixFns maps a token type to its infix handler, populated by the
// expression tasks (3.4, 3.6).
var infixFns = map[lexer.TokenType]infixFn{}

// precOf returns the binding power of the infix operator t, or precNone if t
// is not an infix operator.
func precOf(t lexer.TokenType) int {
	switch t {
	case lexer.TokenAssign, lexer.TokenPlusAssign, lexer.TokenMinusAssign,
		lexer.TokenStarAssign, lexer.TokenSlashAssign, lexer.TokenPercentAssign,
		lexer.TokenAmpAssign, lexer.TokenPipeAssign, lexer.TokenCaretAssign,
		lexer.TokenShlAssign, lexer.TokenShrAssign:
		return precAssign
	case lexer.TokenPipeArrow:
		return precPipe
	case lexer.TokenOr:
		return precOr
	case lexer.TokenAnd:
		return precAnd
	case lexer.TokenEqEq, lexer.TokenBangEq, lexer.TokenLt, lexer.TokenGt,
		lexer.TokenLtEq, lexer.TokenGtEq:
		return precCompare
	case lexer.TokenDotDot, lexer.TokenDotDotEq:
		return precRange
	case lexer.TokenPipe:
		return precBitOr
	case lexer.TokenCaret:
		return precBitXor
	case lexer.TokenAmp:
		return precBitAnd
	case lexer.TokenShl, lexer.TokenShr:
		return precShift
	case lexer.TokenPlus, lexer.TokenMinus:
		return precAdd
	case lexer.TokenStar, lexer.TokenSlash, lexer.TokenPercent:
		return precMul
	case lexer.TokenAs, lexer.TokenIs:
		return precCast
	case lexer.TokenStarStar:
		return precPower
	}
	return precNone
}

// isRightAssoc reports whether t is right-associative: the power operator and
// every assignment operator.
func isRightAssoc(t lexer.TokenType) bool {
	return t == lexer.TokenStarStar || isAssignOp(t)
}

// isNonAssoc reports whether t is non-associative (.. and ..=): chaining two of
// them is a syntax error.
func isNonAssoc(t lexer.TokenType) bool {
	return t == lexer.TokenDotDot || t == lexer.TokenDotDotEq
}

// isAssignOp reports whether t is = or one of the compound assignment
// operators.
func isAssignOp(t lexer.TokenType) bool {
	switch t {
	case lexer.TokenAssign, lexer.TokenPlusAssign, lexer.TokenMinusAssign,
		lexer.TokenStarAssign, lexer.TokenSlashAssign, lexer.TokenPercentAssign,
		lexer.TokenAmpAssign, lexer.TokenPipeAssign, lexer.TokenCaretAssign,
		lexer.TokenShlAssign, lexer.TokenShrAssign:
		return true
	}
	return false
}

// startsType reports whether t can begin a type. Used by the type-vs-expression
// lookahead and by parseType dispatch.
func startsType(t lexer.TokenType) bool {
	if t.IsKeyword() {
		switch t {
		case lexer.TokenFn, lexer.TokenSelfType, lexer.TokenDyn, lexer.TokenWeak,
			lexer.TokenMut:
			return true
		}
	}
	switch t {
	case lexer.TokenIdent, lexer.TokenStar, lexer.TokenLBracket,
		lexer.TokenLParen:
		return true
	}
	return false
}

// startsExpr reports whether t can begin a prefix expression. Used by the
// Pratt driver to decide whether a token is a valid nud.
func startsExpr(t lexer.TokenType) bool {
	if _, ok := prefixFns[t]; ok {
		return true
	}
	switch t {
	case lexer.TokenIdent, lexer.TokenUnderscore, lexer.TokenInt,
		lexer.TokenFloat, lexer.TokenString, lexer.TokenRawString,
		lexer.TokenTrue, lexer.TokenFalse, lexer.TokenNil,
		lexer.TokenLParen, lexer.TokenLBracket, lexer.TokenMinus,
		lexer.TokenPlus, lexer.TokenTilde, lexer.TokenNot,
		lexer.TokenAwait, lexer.TokenFn, lexer.TokenIf, lexer.TokenMatch,
		lexer.TokenSpawn, lexer.TokenSelf, lexer.TokenAt, lexer.TokenLBrace:
		return true
	}
	return false
}

// startsStmt reports whether t can begin a statement. Used by parseBlock to
// detect an incomplete statement and by recovery to find a resync point.
func startsStmt(t lexer.TokenType) bool {
	if startsExpr(t) {
		return true
	}
	switch t {
	case lexer.TokenReturn, lexer.TokenBreak, lexer.TokenContinue,
		lexer.TokenDefer, lexer.TokenConst, lexer.TokenFor,
		lexer.TokenAt, lexer.TokenPub:
		return true
	}
	return false
}

// startsDecl reports whether t can begin a top-level declaration. Used by
// parseProgram and syncDecl.
func startsDecl(t lexer.TokenType) bool {
	switch t {
	case lexer.TokenFn, lexer.TokenStruct, lexer.TokenTrait, lexer.TokenImpl,
		lexer.TokenEnum, lexer.TokenImport, lexer.TokenConst, lexer.TokenPub,
		lexer.TokenAt:
		return true
	}
	return false
}

// declStartTokens is the set of tokens that begin a top-level declaration,
// exported for use by the declaration parser.
var declStartTokens = []lexer.TokenType{
	lexer.TokenFn, lexer.TokenStruct, lexer.TokenTrait, lexer.TokenImpl,
	lexer.TokenEnum, lexer.TokenImport, lexer.TokenConst, lexer.TokenPub,
	lexer.TokenAt,
}

// stmtSyncTokens is the set of tokens that can resynchronize after a
// statement error.
var stmtSyncTokens = []lexer.TokenType{
	lexer.TokenIf, lexer.TokenFor, lexer.TokenMatch, lexer.TokenReturn,
	lexer.TokenBreak, lexer.TokenContinue, lexer.TokenDefer, lexer.TokenSpawn,
	lexer.TokenConst, lexer.TokenStruct, lexer.TokenTrait, lexer.TokenImpl,
	lexer.TokenEnum, lexer.TokenImport, lexer.TokenFrom, lexer.TokenAt,
}

// declSyncTokens is the set of tokens that can resynchronize after a
// declaration error.
var declSyncTokens = []lexer.TokenType{
	lexer.TokenFn, lexer.TokenStruct, lexer.TokenTrait, lexer.TokenImpl,
	lexer.TokenEnum, lexer.TokenImport, lexer.TokenFrom, lexer.TokenConst,
	lexer.TokenPub, lexer.TokenAt,
}
