package plan

import (
	"io"
	"reflect"

	opentracing "github.com/opentracing/opentracing-go"

	"github.com/dolthub/go-mysql-server/sql"
)

// CrossJoin is a cross join between two tables.
type CrossJoin struct {
	BinaryNode
}

// NewCrossJoin creates a new cross join node from two tables.
func NewCrossJoin(left sql.Node, right sql.Node) *CrossJoin {
	return &CrossJoin{
		BinaryNode: BinaryNode{
			left:  left,
			right: right,
		},
	}
}

// Schema implements the Node interface.
func (p *CrossJoin) Schema() sql.Schema {
	return append(p.left.Schema(), p.right.Schema()...)
}

// Resolved implements the Resolvable interface.
func (p *CrossJoin) Resolved() bool {
	return p.left.Resolved() && p.right.Resolved()
}

// RowIter implements the Node interface.
func (p *CrossJoin) RowIter(ctx *sql.Context, row sql.Row) (sql.RowIter, error) {
	var left, right string
	if leftTable, ok := p.left.(sql.Nameable); ok {
		left = leftTable.Name()
	} else {
		left = reflect.TypeOf(p.left).String()
	}

	if rightTable, ok := p.right.(sql.Nameable); ok {
		right = rightTable.Name()
	} else {
		right = reflect.TypeOf(p.right).String()
	}

	span, ctx := ctx.Span("plan.CrossJoin", opentracing.Tags{
		"left":  left,
		"right": right,
	})

	li, err := p.left.RowIter(ctx, row)
	if err != nil {
		span.Finish()
		return nil, err
	}

	return sql.NewSpanIter(span, &crossJoinIterator{
		l:  li,
		rp: p.right,
		s:  ctx,
	}), nil
}

// WithChildren implements the Node interface.
func (p *CrossJoin) WithChildren(children ...sql.Node) (sql.Node, error) {
	if len(children) != 2 {
		return nil, sql.ErrInvalidChildrenNumber.New(p, len(children), 2)
	}

	return NewCrossJoin(children[0], children[1]), nil
}

func (p *CrossJoin) String() string {
	pr := sql.NewTreePrinter()
	_ = pr.WriteNode("CrossJoin")
	_ = pr.WriteChildren(p.left.String(), p.right.String())
	return pr.String()
}

func (p *CrossJoin) DebugString() string {
	pr := sql.NewTreePrinter()
	_ = pr.WriteNode("CrossJoin")
	_ = pr.WriteChildren(sql.DebugString(p.left), sql.DebugString(p.right))
	return pr.String()
}

type rowIterProvider interface {
	RowIter(*sql.Context, sql.Row) (sql.RowIter, error)
}

type crossJoinIterator struct {
	l  sql.RowIter
	rp rowIterProvider
	r  sql.RowIter
	s  *sql.Context

	leftRow sql.Row
}

func (i *crossJoinIterator) Next() (sql.Row, error) {
	for {
		if i.leftRow == nil {
			r, err := i.l.Next()
			if err != nil {
				return nil, err
			}

			i.leftRow = r
		}

		if i.r == nil {
			iter, err := i.rp.RowIter(i.s, i.leftRow)
			if err != nil {
				return nil, err
			}

			i.r = iter
		}

		rightRow, err := i.r.Next()
		if err == io.EOF {
			i.r = nil
			i.leftRow = nil
			continue
		}

		if err != nil {
			return nil, err
		}

		var row sql.Row
		row = append(row, i.leftRow...)
		row = append(row, rightRow...)

		return row, nil
	}
}

func (i *crossJoinIterator) Close() (err error) {
	if i.l != nil {
		err = i.l.Close()
	}

	if i.r != nil {
		if err == nil {
			err = i.r.Close()
		} else {
			i.r.Close()
		}
	}

	return err
}
