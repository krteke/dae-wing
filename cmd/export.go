/*
 * SPDX-License-Identifier: AGPL-3.0-only
 * Copyright (c) 2023, daeuniverse Organization <team@v2raya.org>
 */

package cmd

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/daeuniverse/dae-wing/dae"
	"github.com/daeuniverse/dae-wing/db"
	"github.com/daeuniverse/dae-wing/graphql"
	"github.com/daeuniverse/dae-wing/graphql/service/config"
	"github.com/daeuniverse/dae-wing/pkg/sqlite"
	daeConfig "github.com/daeuniverse/dae/config"
	jsoniter "github.com/json-iterator/go"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var (
	exportConfigDir string
	exportDBFile    string
	exportOutput    string

	exportCmd = &cobra.Command{
		Use:   "export",
		Short: "Export the selected dae configuration or development information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if exportOutput == "" {
				if cmd.Flags().NFlag() == 0 {
					return cmd.Help()
				}
				return fmt.Errorf("-o/--output is required")
			}
			return exportSelectedConfig(exportConfigDir, exportDBFile, exportOutput)
		},
	}
	exportSchemaCmd = &cobra.Command{
		Use: "schema",
		Run: func(cmd *cobra.Command, args []string) {
			schema, err := graphql.SchemaString()
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			fmt.Println(schema)
		},
	}
	exportOutlineCmd = &cobra.Command{
		Use: "outline",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(daeConfig.ExportOutlineJson(db.AppVersion))
		},
	}
	exportFlatDescCmd = &cobra.Command{
		Use: "flatdesc",
		Run: func(cmd *cobra.Command, args []string) {
			b, _ := jsoniter.MarshalIndent(map[string]interface{}{
				"Version": db.AppVersion,
				"Desc":    dae.ExportFlatDesc(),
			}, "", "  ")
			fmt.Println(string(b))
		},
	}
)

func init() {
	exportCmd.Flags().StringVarP(&exportConfigDir, "config", "c", filepath.Join("/etc", db.AppName), "config directory containing wing.db")
	exportCmd.Flags().StringVar(&exportDBFile, "db", "", "path to a wing.db file (overrides --config)")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "output .dae file")
	exportCmd.AddCommand(exportSchemaCmd)
	exportCmd.AddCommand(exportOutlineCmd)
	exportCmd.AddCommand(exportFlatDescCmd)
}

func exportSelectedConfig(configDir, dbFile, output string) error {
	if dbFile == "" {
		if configDir == "" {
			return fmt.Errorf("-c/--config or --db is required")
		}
		dbFile = filepath.Join(configDir, "wing.db")
	}
	dsn := (&url.URL{Scheme: "file", Path: dbFile, RawQuery: "mode=ro"}).String()
	d, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("open database %q: %w", dbFile, err)
	}
	sqlDB, err := d.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	tx := d.Begin(&sql.TxOptions{ReadOnly: true})
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()
	prepared, err := config.BuildConfig(tx)
	if err != nil {
		return fmt.Errorf("build selected configuration: %w", err)
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}
	content, err := prepared.Config.Marshal(2)
	if err != nil {
		return fmt.Errorf("marshal dae configuration: %w", err)
	}
	if err := os.WriteFile(output, content, 0600); err != nil {
		return fmt.Errorf("write %q: %w", output, err)
	}
	return nil
}
