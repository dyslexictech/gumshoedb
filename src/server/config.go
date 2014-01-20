package main

import (
	"errors"
	"time"
)

type Config struct {
	TableFilePath string   `toml:"table_file_path"`
	// TODO(caleb): I added the save duration field because it was a constant before the configuration file
	// existed, but it's currently unused.
	SaveDuration  duration `toml:"save_duration"`
	ColumnNames   []string `toml:"column_names"`
}

func (c *Config) Validate() error {
	if len(c.ColumnNames) == 0 {
		return errors.New("Must provide at least one column name in the configuration.")
	}
	return nil
}

type duration struct {
	time.Duration
}

func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}
