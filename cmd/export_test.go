package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/daeuniverse/dae-wing/db"
	daeConfig "github.com/daeuniverse/dae/config"
	"github.com/daeuniverse/dae/pkg/config_parser"
)

func TestExportSelectedConfig(t *testing.T) {
	configDir := t.TempDir()
	if err := db.InitDatabase(configDir); err != nil {
		t.Fatal(err)
	}
	d := db.DB(context.Background())
	for _, record := range []interface{}{
		&db.Config{Global: "global {}", Selected: true},
		&db.Dns{Dns: "dns {}", Selected: true},
		&db.Routing{Routing: "routing { fallback: proxy }", Selected: true},
	} {
		if err := d.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
	group := db.Group{Name: "proxy", Policy: "fixed"}
	node := db.Node{Name: "test", Link: "socks5://127.0.0.1:1080"}
	if err := d.Create(&group).Error; err != nil {
		t.Fatal(err)
	}
	if err := d.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	if err := d.Model(&group).Association("Node").Append(&node); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name      string
		configDir string
		dbFile    string
	}{
		{name: "config directory", configDir: configDir},
		{name: "explicit database", configDir: filepath.Join(configDir, "missing"), dbFile: filepath.Join(configDir, "wing.db")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "export.dae")
			args := []string{"export", "-c", tc.configDir, "-o", output}
			if tc.dbFile != "" {
				args = append(args, "--db", tc.dbFile)
			}
			rootCmd.SetArgs(args)
			if err := rootCmd.Execute(); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			sections, err := config_parser.Parse(string(content))
			if err != nil {
				t.Fatal(err)
			}
			conf, err := daeConfig.New(sections)
			if err != nil {
				t.Fatal(err)
			}
			if len(conf.Group) != 1 || conf.Group[0].Name != "proxy" || len(conf.Node) != 1 || string(conf.Node[0]) != "test:socks5://127.0.0.1:1080" {
				t.Fatalf("unexpected exported group or node: groups=%+v nodes=%+v", conf.Group, conf.Node)
			}
		})
	}
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
}
