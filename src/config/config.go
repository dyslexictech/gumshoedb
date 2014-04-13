package config

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"gumshoe"

	"github.com/BurntSushi/toml"
)

// All struct fields with a toml tag are required (see checkUndefinedFields).

type Schema struct {
	IntervalDuration Duration    `toml:"interval_duration"`
	TimestampColumn  [2]string   `toml:"timestamp_column"`
	DimensionColumns [][2]string `toml:"dimension_columns"`
	MetricColumns    [][2]string `toml:"metric_columns"`
}

type Config struct {
	ListenAddr    string   `toml:"listen_addr"`
	StatsdAddr    string   `toml:"statsd_addr"`
	DatabaseDir   string   `toml:"database_dir"`
	FlushInterval Duration `toml:"flush_interval"`
	RetentionDays int      `toml:"retention_days"`
	Schema        Schema   `toml:"schema"`
}

// Produces a gumshoe Schema based on a Config's values.
func (c *Config) makeSchema() (*gumshoe.Schema, error) {
	dir := ""
	diskBacked := true
	switch c.DatabaseDir {
	case "":
		return nil, errors.New("database directory must be provided. Use 'MEMORY' to specify an in-memory DB.")
	case "MEMORY":
		diskBacked = false
	default:
		dir = c.DatabaseDir
	}

	name, typ, isString := parseColumn(c.Schema.TimestampColumn)
	if typ != "uint32" {
		return nil, fmt.Errorf("timestamp column (%q) must be uint32", name)
	}
	if isString {
		return nil, errors.New("timestamp column cannot be a string")
	}
	timestampColumn, err := gumshoe.MakeDimensionColumn(name, typ, isString)
	if err != nil {
		return nil, err
	}

	dimensions := make([]gumshoe.DimensionColumn, len(c.Schema.DimensionColumns))
	for i, colPair := range c.Schema.DimensionColumns {
		name, typ, isString := parseColumn(colPair)
		if isString {
			switch typ {
			case "uint8", "uint16", "uint32":
			default:
				return nil, fmt.Errorf("got type %q for column %q (must be unsigned int type)", typ, name)
			}
		}
		col, err := gumshoe.MakeDimensionColumn(name, typ, isString)
		if err != nil {
			return nil, err
		}
		dimensions[i] = col
	}

	if len(c.Schema.MetricColumns) == 0 {
		return nil, fmt.Errorf("schema must include at least one metric column")
	}
	metrics := make([]gumshoe.MetricColumn, len(c.Schema.MetricColumns))
	for i, colPair := range c.Schema.MetricColumns {
		name, typ, isString := parseColumn(colPair)
		if isString {
			return nil, fmt.Errorf("metric column (%q) has string type; not allowed for metric columns", name)
		}
		col, err := gumshoe.MakeMetricColumn(name, typ)
		if err != nil {
			return nil, err
		}
		metrics[i] = col
	}

	// Check that we haven't duplicated any column names
	names := map[string]bool{timestampColumn.Name: true}
	for _, col := range dimensions {
		if names[col.Name] {
			return nil, fmt.Errorf("duplicate column name %q", col.Name)
		}
		names[col.Name] = true
	}
	for _, col := range metrics {
		if names[col.Name] {
			return nil, fmt.Errorf("duplicate column name %q", col.Name)
		}
		names[col.Name] = true
	}

	// Check durations for sanity
	if c.FlushInterval.Duration < time.Second {
		return nil, fmt.Errorf("flush interval is too small: %s", c.FlushInterval)
	}
	if c.RetentionDays < 1 {
		return nil, fmt.Errorf("retention days is too small: %d", c.RetentionDays)
	}
	if c.Schema.IntervalDuration.Duration < time.Minute {
		return nil, fmt.Errorf("interval duration is too short: %s", c.Schema.IntervalDuration)
	}

	return &gumshoe.Schema{
		TimestampColumn:  timestampColumn.Column,
		DimensionColumns: dimensions,
		MetricColumns:    metrics,
		SegmentSize:      1e6,
		IntervalDuration: c.Schema.IntervalDuration.Duration,
		DiskBacked:       diskBacked,
		Dir:              dir,
		RunConfig: gumshoe.RunConfig{
			FixedRetention: true,
			Retention:      time.Duration(c.RetentionDays) * 24 * time.Hour,
		},
	}, nil
}

func parseColumn(col [2]string) (name, typ string, isString bool) {
	name = col[0]
	typ = col[1]
	if strings.HasPrefix(typ, "string:") {
		typ = strings.TrimPrefix(typ, "string:")
		isString = true
	}
	return
}

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

func (d Duration) MarshalText() ([]byte, error) { return []byte(d.Duration.String()), nil }

func LoadTOMLConfig(r io.Reader) (*Config, *gumshoe.Schema, error) {
	config := new(Config)
	meta, err := toml.DecodeReader(r, config)
	if err != nil {
		return nil, nil, err
	}
	if err := checkUndefinedFields(meta, config); err != nil {
		return nil, nil, err
	}
	schema, err := config.makeSchema()
	if err != nil {
		return nil, nil, err
	}
	return config, schema, nil
}
