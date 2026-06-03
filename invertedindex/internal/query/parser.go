package query

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

func ParseQuery(queryStr string) (*Node, error) {
	processed := strings.ToUpper(queryStr)

	result, err := queryParser.ParseString("", processed)
	if err != nil {
		return nil, fmt.Errorf("parse query %q: %w", queryStr, err)
	}
	return convertQuery(result), nil
}

var queryLexer = lexer.MustSimple([]lexer.SimpleRule{
	{"Whitespace", `\s+`},
	{"And", `\bAND\b`},
	{"Or", `\bOR\b`},
	{"Not", `\bNOT\b`},
	{"Adj", `\bADJ\b`},
	{"NearDist", `\bNEAR/\d+\b`},
	{"Word", `[A-Z0-9_]+`},
	{"Lparen", `\(`},
	{"Rparen", `\)`},
})

type pQuery struct {
	Expr *pOrExpr `@@`
}

type pOrExpr struct {
	Left  *pAndExpr   `@@`
	Right []*pAndExpr `("OR" @@)*`
}

type pAndExpr struct {
	Left  *pUnaryExpr    `@@`
	Right []*pAndOperand `@@*`
}

type pAndOperand struct {
	Op   string      `@(And|Adj|NearDist)`
	Expr *pUnaryExpr `@@`
}

type pUnaryExpr struct {
	Negated *pPrimary `"NOT" @@`
	Pos     *pPrimary `| @@`
}

type pPrimary struct {
	Term     string  `@Word`
	Subquery *pQuery `| "(" @@ ")"`
}

var queryParser = participle.MustBuild[pQuery](
	participle.Lexer(queryLexer),
	participle.Elide("Whitespace"),
)

func convertQuery(q *pQuery) *Node {
	if q == nil || q.Expr == nil {
		return nil
	}
	return convertOrExpr(q.Expr)
}

func convertOrExpr(e *pOrExpr) *Node {
	left := convertAndExpr(e.Left)
	for _, right := range e.Right {
		left = newOrNode(left, convertAndExpr(right))
	}
	return left
}

func convertAndExpr(e *pAndExpr) *Node {
	left := convertUnaryExpr(e.Left)
	for _, r := range e.Right {
		right := convertUnaryExpr(r.Expr)
		op := r.Op
		if strings.HasPrefix(op, "NEAR") {
			distance := parseNearDistance(op)
			left = newNearNode(left, right, distance)
		} else {
			switch op {
			case "AND":
				left = newAndNode(left, right)
			case "ADJ":
				left = newAdjNode(left, right)
			}
		}
	}
	return left
}

func convertUnaryExpr(e *pUnaryExpr) *Node {
	if e.Negated != nil {
		return newNotNode(convertPrimary(e.Negated))
	}
	return convertPrimary(e.Pos)
}

func convertPrimary(p *pPrimary) *Node {
	if p.Subquery != nil {
		return convertQuery(p.Subquery)
	}
	return newTermNode(strings.ToLower(p.Term))
}

func parseNearDistance(op string) int {
	if strings.Contains(op, "/") {
		parts := strings.SplitN(op, "/", 2)
		if len(parts) == 2 {
			if d, err := strconv.Atoi(parts[1]); err == nil {
				return d
			}
		}
	}
	return 1
}
