package gumshoe

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/philc/gumshoedb/internal/util"

	. "github.com/philc/gumshoedb/internal/github.com/cespare/a"
)

func TestExpectedNumberOfSegmentsAreAllocated(t *testing.T) {
	db := makeTestDB()
	defer closeTestDB(db)
	db.Schema.SegmentSize = 32 // Rows are 13 bytes apiece

	insertRows(db, []RowMap{
		{"at": 0.0, "dim1": "a", "metric1": 1.0},
		{"at": 0.0, "dim1": "b", "metric1": 1.0},
		{"at": 0.0, "dim1": "c", "metric1": 1.0},
	})

	resp := db.MakeRequest()
	defer resp.Done()

	Assert(t, len(resp.StaticTable.Intervals), Equals, 1)
	numSegments := 0
	for _, interval := range resp.StaticTable.Intervals {
		numSegments += interval.NumSegments
	}
	Assert(t, numSegments, Equals, 2)
}

func TestRowsGetCollapsedUponInsertion(t *testing.T) {
	db := makeTestDB()
	defer closeTestDB(db)

	rows := []RowMap{
		// These two rows should be collapsed
		{"at": 0.0, "dim1": "string1", "metric1": 1.0},
		{"at": 0.0, "dim1": "string1", "metric1": 3.0},
		// This row should not, because it has a nil column.
		{"at": 0.0, "dim1": nil, "metric1": 5.0},
		// This row should not be collapsed with the others, because it falls in a different time interval.
		{"at": hour(2), "dim1": "string1", "metric1": 7.0},
	}

	insertRows(db, rows)
	Assert(t, db.GetDebugRows(), util.DeepEqualsUnordered, []UnpackedRow{
		{RowMap: RowMap{"at": 0.0, "dim1": "string1", "metric1": 4}, Count: 2},
		{RowMap: rows[2], Count: 1},
		{RowMap: rows[3], Count: 1},
	})
}

type Time time.Time

func (t Time) hoursBack(n int) float64 {
	return float64(time.Time(t).Add(time.Duration(-n) * time.Hour).Unix())
}

func TestMemAndStaticIntervalsAreCombined(t *testing.T) {
	db := makeTestDB()
	defer closeTestDB(db)

	start := Time(time.Now())

	insertRows(db, []RowMap{
		{"at": start.hoursBack(0), "dim1": "string1", "metric1": 1.0},
		{"at": start.hoursBack(1), "dim1": "string1", "metric1": 1.0},
	})
	Assert(t, len(db.StaticTable.Intervals), Equals, 2)

	insertRows(db, []RowMap{
		{"at": start.hoursBack(1), "dim1": "string1", "metric1": 1.0},
		{"at": start.hoursBack(2), "dim1": "string1", "metric1": 1.0},
	})
	Assert(t, len(db.StaticTable.Intervals), Equals, 3)

	Assert(t, db.GetDebugRows(), util.DeepEqualsUnordered, []UnpackedRow{
		{RowMap: RowMap{"at": start.hoursBack(0), "dim1": "string1", "metric1": 1}, Count: 1},
		{RowMap: RowMap{"at": start.hoursBack(1), "dim1": "string1", "metric1": 2}, Count: 2},
		{RowMap: RowMap{"at": start.hoursBack(2), "dim1": "string1", "metric1": 1}, Count: 1},
	})
}

func TestDeleteSegmentsOutOfRetention(t *testing.T) {
	db := makeTestDB()
	db.FixedRetention = false // Turn this on midway; kinda hacky but easy
	db.Retention = 24 * time.Hour
	defer closeTestDB(db)

	start := Time(time.Now())

	insertRows(db, []RowMap{
		{"at": start.hoursBack(36), "dim1": "string1", "metric1": 1.0},
		{"at": start.hoursBack(12), "dim1": "string1", "metric1": 1.0},
	})
	db.FixedRetention = true
	insertRows(db, []RowMap{
		{"at": start.hoursBack(36), "dim1": "string1", "metric1": 1.0},
		{"at": start.hoursBack(12), "dim1": "string1", "metric1": 1.0},
	})

	Assert(t, db.GetDebugRows(), util.DeepEqualsUnordered, []UnpackedRow{
		{RowMap: RowMap{"at": start.hoursBack(12), "dim1": "string1", "metric1": 2}, Count: 2},
	})
}

func TestInsertAndReadNilValues(t *testing.T) {
	db := makeTestDB()
	defer closeTestDB(db)

	rows := []RowMap{
		{"at": hour(0), "dim1": "a", "metric1": 0.0},
		{"at": hour(1), "dim1": nil, "metric1": 1.0},
	}
	insertRows(db, rows)
	Assert(t, db.GetDebugRows(), util.DeepEqualsUnordered, []UnpackedRow{{rows[0], 1}, {rows[1], 1}})
}

func TestInsertOverflow(t *testing.T) {
	schema := schemaFixture()
	schema.DimensionColumns = []DimensionColumn{makeDimensionColumn("dim1", "uint8", true)}
	schema.MetricColumns = []MetricColumn{makeMetricColumn("metric1", "uint8")}
	db, err := NewDB(schema)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	var rows []RowMap
	for i := 0; i < 257; i++ {
		rows = append(rows, RowMap{"at": hour(0), "dim1": strconv.Itoa(i), "metric1": 1.0})
	}
	Assert(t, db.Insert(rows), NotNil)
	rows = []RowMap{{"at": hour(0), "dim1": "0", "metric1": 1000.0}}
	Assert(t, db.Insert(rows), NotNil)
}

func TestInsertDropsRowsOutOfRetention(t *testing.T) {
	db := makeTestDB()
	db.FixedRetention = true
	db.Retention = 24 * time.Hour
	defer closeTestDB(db)

	rows := []RowMap{
		{"at": float64(time.Now().Add(-22 * time.Hour).Unix()), "dim1": "foo", "metric1": 1.0},
		{"at": float64(time.Now().Add(-26 * time.Hour).Unix()), "dim1": "bar", "metric1": 1.0},
	}

	insertRows(db, rows)
	Assert(t, db.GetDebugRows(), util.DeepEqualsUnordered, []UnpackedRow{{rows[0], 1}})
}

func makeTestPersistentDB() *DB {
	tempDir, err := ioutil.TempDir("", "gumshoe-persistence-test")
	if err != nil {
		panic(err)
	}
	schema := schemaFixture()
	schema.DiskBacked = true
	schema.Dir = tempDir
	db, err := NewDB(schema)
	if err != nil {
		panic(err)
	}
	return db
}

func physicalRows(db *DB) int {
	resp := db.MakeRequest()
	defer resp.Done()
	rows := 0
	for _, interval := range resp.StaticTable.Intervals {
		rows += interval.NumRows
	}
	return rows
}

func TestPersistenceEndToEnd(t *testing.T) {
	db := makeTestPersistentDB()
	defer os.RemoveAll(db.Dir)

	// Insert a bunch of data
	var rows []RowMap
	for i := 0; i < 10000; i++ {
		// Generate 100 unique rows.
		rows = append(rows, RowMap{"at": 0.0, "dim1": strconv.Itoa(i % 100), "metric1": 1.0})
	}
	insertRows(db, rows)

	// Query the data
	Assert(t, physicalRows(db), Equals, 100)
	result := runQuery(db, createQuery())
	Assert(t, result[0]["metric1"], util.DeepConvertibleEquals, 10000)

	// Reopen the DB and try again
	db = reopenTestDB(db)
	defer closeTestDB(db)
	Assert(t, physicalRows(db), Equals, 100)
	result = runQuery(db, createQuery())
	Assert(t, result[0]["metric1"], util.DeepConvertibleEquals, 10000)
}

func TestOldIntervalsAreDeleted(t *testing.T) {
	db := makeTestPersistentDB()
	defer os.RemoveAll(db.Dir)
	defer closeTestDB(db)

	insertRow(db, RowMap{"at": 0.0, "dim1": "string1", "metric1": 1.0})

	firstGenSegmentFilename := filepath.Join(db.Dir, "interval.0.generation0000.segment0000.dat")
	if _, err := os.Stat(firstGenSegmentFilename); err != nil {
		t.Fatalf("expected segment file at %s to exist", firstGenSegmentFilename)
	}

	insertRow(db, RowMap{"at": 0.0, "dim1": "string1", "metric1": 1.0})
	if _, err := os.Stat(firstGenSegmentFilename); !os.IsNotExist(err) {
		t.Fatalf("expected segment file at %s to have been deleted", firstGenSegmentFilename)
	}
	secondGenSegmentFilename := filepath.Join(db.Dir, "interval.0.generation0001.segment0000.dat")
	if _, err := os.Stat(secondGenSegmentFilename); err != nil {
		t.Fatalf("expected segment file at %s to exist", secondGenSegmentFilename)
	}
}
