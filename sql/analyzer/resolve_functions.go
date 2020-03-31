package analyzer

import (
	"github.com/src-d/go-mysql-server/sql"
	"github.com/src-d/go-mysql-server/sql/expression"
	"github.com/src-d/go-mysql-server/sql/plan"
)

func resolveFunctions(ctx *sql.Context, a *Analyzer, n sql.Node) (sql.Node, error) {
	span, _ := ctx.Span("resolve_functions")
	defer span.Finish()

	a.Log("resolve functions, node of type %T", n)
	return plan.TransformUp(n, func(n sql.Node) (sql.Node, error) {
		if n.Resolved() {
			return n, nil
		}

		return plan.TransformExpressionsUp(n, resolveFunctionsInExpr(a))
	})
}

func resolveFunctionsInExpr(a *Analyzer) sql.TransformExprFunc {
	return func(e sql.Expression) (sql.Expression, error) {
		if e.Resolved() {
			return e, nil
		}

		uf, ok := e.(*expression.UnresolvedFunction)
		if !ok {
			return e, nil
		}

		n := uf.Name()
		f, err := a.Catalog.Function(n)
		if err != nil {
			return nil, err
		}

		rf, err := f.Call(uf.Arguments...)
		if err != nil {
			return nil, err
		}

		a.Log("resolved function %q", n)
		return rf, nil
	}
}
