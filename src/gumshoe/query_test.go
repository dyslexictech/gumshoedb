package gumshoe

import (
	"testing"

	"utils"

	. "github.com/cespare/a"
)

func createQuery() *Query {
	return &Query{
		Aggregates: []QueryAggregate{
			{Type: AggregateSum, Column: "metric1", Name: "metric1"},
		},
	}
}

func createTestDBForFilterTests() *DB {
	db := testDB()
	insertRow(db, 0.0, "string1", 1.0)
	insertRow(db, 0.0, "string2", 2.0)
	db.flush()
	return db
}

func createTestDBForNilQueryTests() *DB {
	db := testDB()
	insertRow(db, 0.0, "a", 1.0)
	insertRow(db, 0.0, "b", 2.0)
	insertRow(db, 0.0, nil, 4.0)
	db.flush()
	return db
}

func runQuery(db *DB, query *Query) []RowMap {
	results, err := db.GetQueryResult(query)
	if err != nil {
		panic(err)
	}
	return results
}

func runWithFilter(db *DB, filter QueryFilter) []RowMap {
	query := createQuery()
	query.Filters = []QueryFilter{filter}
	return runQuery(db, query)
}

func runWithGroupBy(db *DB, grouping QueryGrouping) []RowMap {
	query := createQuery()
	query.Groupings = []QueryGrouping{grouping}
	return runQuery(db, query)
}

func TestInvokeQueryFiltersRowsUsingEqualsFilter(t *testing.T) {
	db := createTestDBForFilterTests()
	defer db.Close()

	results := runWithFilter(db, QueryFilter{FilterEqual, "metric1", 2.0})
	Assert(t, results[0]["metric1"], Equals, uint32(2))

	results = runWithFilter(db, QueryFilter{FilterEqual, "dim1", "string2"})
	Assert(t, results[0]["metric1"], Equals, uint32(2))

	results = runWithFilter(db, QueryFilter{FilterEqual, "at", 0.0})
	Assert(t, results[0]["metric1"], Equals, uint32(3))

	results = runWithFilter(db, QueryFilter{FilterEqual, "at", 1.0})
	Assert(t, results[0]["metric1"], Equals, uint32(0))

	// These match zero rows.
	results = runWithFilter(db, QueryFilter{FilterEqual, "metric1", 3.0})
	Assert(t, results[0]["metric1"], Equals, uint32(0))

	results = runWithFilter(db, QueryFilter{FilterEqual, "dim1", "non-existent"})
	Assert(t, results[0]["metric1"], Equals, uint32(0))
}

func TestInvokeQueryFiltersRowsUsingLessThan(t *testing.T) {
	db := createTestDBForFilterTests()
	defer db.Close()

	results := runWithFilter(db, QueryFilter{FilterLessThan, "metric1", 2.0})
	Assert(t, results[0]["metric1"], Equals, uint32(1))

	results = runWithFilter(db, QueryFilter{FilterLessThan, "at", 10.0})
	Assert(t, results[0]["metric1"], Equals, uint32(3))

	// Matches zero rows.
	results = runWithFilter(db, QueryFilter{FilterLessThan, "metric1", 1.0})
	Assert(t, results[0]["metric1"], Equals, uint32(0))
}

func inList(items ...interface{}) []interface{} {
	list := make([]interface{}, len(items))
	for i, item := range items {
		switch typed := item.(type) {
		case string, float64, nil:
			list[i] = typed
		case int:
			list[i] = float64(typed)
		default:
			panic("bad type")
		}
	}
	return list
}

func TestInvokeQueryFiltersRowsUsingIn(t *testing.T) {
	db := createTestDBForFilterTests()

	Assert(t, runWithFilter(db, QueryFilter{FilterIn, "metric1", inList(2)})[0]["metric1"], Equals, uint32(2))
	results := runWithFilter(db, QueryFilter{FilterIn, "metric1", inList(2, 1)})
	Assert(t, results[0]["metric1"], Equals, uint32(3))

	results = runWithFilter(db, QueryFilter{FilterIn, "dim1", inList("string1")})
	Assert(t, results[0]["metric1"], Equals, uint32(1))

	results = runWithFilter(db, QueryFilter{FilterIn, "at", inList(0, 10, 100)})
	Assert(t, results[0]["metric1"], Equals, uint32(3))

	// These match zero rows.
	results = runWithFilter(db, QueryFilter{FilterIn, "metric1", inList(3)})
	Assert(t, results[0]["metric1"], Equals, uint32(0))
	results = runWithFilter(db, QueryFilter{FilterIn, "dim1", inList("non-existent")})
	Assert(t, results[0]["metric1"], Equals, uint32(0))
}

func TestInvokeQueryWorksWhenGroupingByAStringColumn(t *testing.T) {
	db := testDB()
	defer db.Close()
	insertRow(db, 0.0, "string1", 1.0)
	insertRow(db, 0.0, "string1", 2.0)
	insertRow(db, 0.0, "string2", 5.0)
	db.Flush()

	result := runWithGroupBy(db, QueryGrouping{TimeTruncationNone, "dim1", "groupbykey"})
	Assert(t, result[0], utils.HasEqualJSON, RowMap{"groupbykey": "string1", "rowCount": 2, "metric1": 3})
	Assert(t, result[1], utils.HasEqualJSON, RowMap{"groupbykey": "string2", "rowCount": 1, "metric1": 5})
}

func TestGroupingWithATimeTransformFunctionWorks(t *testing.T) {
	db := testDB()
	defer db.Close()
	// at is truncated by day, so these points are from day 0, 2, 2.
	const twoDays = 2 * 24 * 60 * 60
	insertRow(db, 0.0, "", 0.0)
	insertRow(db, float64(twoDays), "", 10.0)
	insertRow(db, twoDays+100.0, "", 12.0)
	db.Flush()

	result := runWithGroupBy(db, QueryGrouping{TimeTruncationDay, "at", "groupbykey"})
	Assert(t, result[0], utils.HasEqualJSON, RowMap{"groupbykey": 0, "rowCount": 1, "metric1": 0})
	Assert(t, result[1], utils.HasEqualJSON, RowMap{"groupbykey": twoDays, "rowCount": 2, "metric1": 22})
}

func TestAggregateQueryWithNilValues(t *testing.T) {
	db := createTestDBForNilQueryTests()
	defer db.Close()
	results := runQuery(db, createQuery())
	Assert(t, results[0], utils.HasEqualJSON, map[string]Untyped{"metric1": 7, "rowCount": 3})
}

func TestFilterQueryWithNilValues(t *testing.T) {
	db := createTestDBForNilQueryTests()
	defer db.Close()

	results := runWithFilter(db, QueryFilter{FilterEqual, "dim1", "a"})
	Assert(t, results[0], utils.HasEqualJSON, map[string]Untyped{"metric1": 1, "rowCount": 1})

	results = runWithFilter(db, QueryFilter{FilterEqual, "dim1", nil})
	Assert(t, results[0], utils.HasEqualJSON, map[string]Untyped{"metric1": 4, "rowCount": 1})
}

func TestFilterQueryUsingInWithNilValues(t *testing.T) {
	db := createTestDBForNilQueryTests()
	results := runWithFilter(db, QueryFilter{FilterIn, "dim1", inList("b", nil)})
	Assert(t, results[0], utils.HasEqualJSON, map[string]Untyped{"metric1": 6, "rowCount": 2})
}

func TestGroupByQueryWithNilValues(t *testing.T) {
	db := createTestDBForNilQueryTests()
	defer db.Close()
	results := runWithGroupBy(db, QueryGrouping{TimeTruncationNone, "dim1", "groupbykey"})
	Assert(t, results, utils.HasEqualJSON, []RowMap{
		// TODO(caleb): This ordering is a bit fragile -- it could change depending on how we decide to sort
		// dimension bytes.
		{"metric1": 1, "groupbykey": "a", "rowCount": 1},
		{"metric1": 2, "groupbykey": "b", "rowCount": 1},
		{"metric1": 4, "groupbykey": nil, "rowCount": 1},
	})
}
