package analyzer

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dolthub/go-mysql-server/memory"
	"github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/expression"
	"github.com/dolthub/go-mysql-server/sql/plan"
)

func TestResolveTables(t *testing.T) {
	require := require.New(t)
	f := getRule("resolve_tables")

	table := memory.NewTable("mytable", sql.Schema{{Name: "i", Type: sql.Int32}})
	db := memory.NewHistoryDatabase("mydb")
	db.AddTableAsOf("mytable", table, "2019-01-01")

	catalog := sql.NewCatalog()
	catalog.AddDatabase(db)

	a := NewBuilder(catalog).AddPostAnalyzeRule(f.Name, f.Apply).Build()
	ctx := sql.NewEmptyContext().WithCurrentDB("mydb")

	var notAnalyzed sql.Node = plan.NewUnresolvedTable("mytable", "")
	analyzed, err := f.Apply(ctx, a, notAnalyzed, nil)
	require.NoError(err)
	require.Equal(plan.NewResolvedTable(table), analyzed)

	notAnalyzed = plan.NewUnresolvedTable("MyTable", "")
	analyzed, err = f.Apply(ctx, a, notAnalyzed, nil)
	require.NoError(err)
	require.Equal(plan.NewResolvedTable(table), analyzed)

	notAnalyzed = plan.NewUnresolvedTable("nonexistant", "")
	analyzed, err = f.Apply(ctx, a, notAnalyzed, nil)
	require.Error(err)
	require.Nil(analyzed)

	analyzed, err = f.Apply(ctx, a, plan.NewResolvedTable(table), nil)
	require.NoError(err)
	require.Equal(plan.NewResolvedTable(table), analyzed)

	notAnalyzed = plan.NewUnresolvedTable("dual", "")
	analyzed, err = f.Apply(ctx, a, notAnalyzed, nil)
	require.NoError(err)
	require.Equal(plan.NewResolvedTable(dualTable), analyzed)

	notAnalyzed = plan.NewUnresolvedTable("dual", "")
	analyzed, err = f.Apply(ctx, a, notAnalyzed, nil)
	require.NoError(err)
	require.Equal(plan.NewResolvedTable(dualTable), analyzed)

	notAnalyzed = plan.NewUnresolvedTableAsOf("myTable", "", expression.NewLiteral("2019-01-01", sql.LongText))
	analyzed, err = f.Apply(ctx, a, notAnalyzed, nil)
	require.NoError(err)
	require.Equal(plan.NewResolvedTable(table), analyzed)

	notAnalyzed = plan.NewUnresolvedTableAsOf("myTable", "", expression.NewLiteral("2019-01-02", sql.LongText))
	analyzed, err = f.Apply(ctx, a, notAnalyzed, nil)
	require.Error(err)
}

func TestResolveTablesNoCurrentDB(t *testing.T) {
	require := require.New(t)
	f := getRule("resolve_tables")

	table := memory.NewTable("mytable", sql.Schema{{Name: "i", Type: sql.Int32}})
	db := memory.NewDatabase("mydb")
	db.AddTable("mytable", table)

	catalog := sql.NewCatalog()
	catalog.AddDatabase(db)

	a := NewBuilder(catalog).AddPostAnalyzeRule(f.Name, f.Apply).Build()
	ctx := sql.NewEmptyContext()

	var notAnalyzed sql.Node = plan.NewUnresolvedTable("mytable", "")
	_, err := f.Apply(ctx, a, notAnalyzed, nil)
	require.Error(err)
	require.True(sql.ErrNoDatabaseSelected.Is(err), "wrong error kind")

	notAnalyzed = plan.NewUnresolvedTable("mytable", "doesNotExist")
	_, err = f.Apply(ctx, a, notAnalyzed, nil)
	require.Error(err)
	require.True(sql.ErrDatabaseNotFound.Is(err), "wrong error kind")

	notAnalyzed = plan.NewUnresolvedTable("dual", "")
	analyzed, err := f.Apply(ctx, a, notAnalyzed, nil)
	require.NoError(err)
	require.Equal(plan.NewResolvedTable(dualTable), analyzed)
}

func TestResolveTablesNested(t *testing.T) {
	require := require.New(t)

	f := getRule("resolve_tables")

	table := memory.NewTable("mytable", sql.Schema{{Name: "i", Type: sql.Int32}})
	table2 := memory.NewTable("my_other_table", sql.Schema{{Name: "i", Type: sql.Int32}})
	db := memory.NewDatabase("mydb")
	db.AddTable("mytable", table)
	catalog := sql.NewCatalog()
	catalog.AddDatabase(db)

	db2 := memory.NewDatabase("my_other_db")
	db2.AddTable("my_other_table", table2)
	catalog.AddDatabase(db2)

	a := NewBuilder(catalog).AddPostAnalyzeRule(f.Name, f.Apply).Build()
	ctx := sql.NewEmptyContext().WithCurrentDB("mydb")

	notAnalyzed := plan.NewProject(
		[]sql.Expression{expression.NewGetField(0, sql.Int32, "i", true)},
		plan.NewUnresolvedTable("mytable", ""),
	)
	analyzed, err := f.Apply(ctx, a, notAnalyzed, nil)
	require.NoError(err)
	expected := plan.NewProject(
		[]sql.Expression{expression.NewGetField(0, sql.Int32, "i", true)},
		plan.NewResolvedTable(table),
	)
	require.Equal(expected, analyzed)

	notAnalyzed = plan.NewProject(
		[]sql.Expression{expression.NewGetField(0, sql.Int32, "i", true)},
		plan.NewUnresolvedTable("my_other_table", "my_other_db"),
	)
	analyzed, err = f.Apply(ctx, a, notAnalyzed, nil)
	require.NoError(err)
	expected = plan.NewProject(
		[]sql.Expression{expression.NewGetField(0, sql.Int32, "i", true)},
		plan.NewResolvedTable(table2),
	)
	require.Equal(expected, analyzed)
}
