package record

import (
	"archive/tar"
	"database/sql"
	"encoding/json"
	"github.com/lesovsky/pgcenter/internal/postgres"
	"github.com/lesovsky/pgcenter/internal/stat"
	"github.com/lesovsky/pgcenter/internal/view"
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func Test_tarCollector_open_close(t *testing.T) {
	tc := newTarCollector(tarConfig{filename: "/tmp/pgcenter-record-testing.stat.tar", truncate: true})
	assert.NoError(t, tc.open())
	assert.NoError(t, tc.close())

	tc = newTarCollector(tarConfig{filename: "/tmp/pgcenter-record-testing.stat.tar", truncate: false})
	assert.NoError(t, tc.open())
	assert.NoError(t, tc.close())
}

func Test_tarCollector_collect(t *testing.T) {
	tc := newTarCollector(tarConfig{filename: "/tmp/pgcenter-record-testing.stat.tar"})
	assert.NoError(t, tc.open())

	// create and configure views
	db, err := postgres.NewTestConnect()
	views, err := configureViews(db, view.New())
	db.Close()

	// create postgres config
	dbConfig, err := postgres.NewTestConfig()
	assert.NoError(t, err)
	stats, err := tc.collect(dbConfig, views)
	assert.NotNil(t, stats)

	// check all stats have filled columns
	for _, s := range stats {
		assert.NoError(t, s.Err)
		assert.Greater(t, len(s.Cols), 0)
	}

	assert.NoError(t, tc.close())
}

func Test_tarCollector_write(t *testing.T) {
	stats := map[string]stat.PGresult{
		"pgcenter_record_testing": {
			Valid: true, Ncols: 2, Nrows: 4, Cols: []string{"col1", "col2"},
			Values: [][]sql.NullString{
				{{String: "alfa", Valid: true}, {String: "12.06157", Valid: true}},
				{{String: "bravo", Valid: true}, {String: "819.188", Valid: true}},
				{{String: "charli", Valid: true}, {String: "18.126", Valid: true}},
				{{String: "delta", Valid: true}, {String: "137.176", Valid: true}},
			},
		},
	}

	filename := "/tmp/pgcenter-record-testing.stat.tar"

	// Write testdata.
	tc := newTarCollector(tarConfig{filename: filename, truncate: true})
	assert.NoError(t, tc.open())
	assert.NoError(t, tc.write(stats))
	assert.NoError(t, tc.close())

	// Read written testdata and compare with origin testdata.
	f, err := os.Open(filepath.Clean(filename)) // open file
	assert.NoError(t, err)
	assert.NotNil(t, f)

	tr := tar.NewReader(f) // create tar reader
	hdr, err := tr.Next()
	assert.NoError(t, err)
	data := make([]byte, hdr.Size) // make data buffer
	_, err = io.ReadFull(tr, data) // read data from tar to buffer
	assert.NoError(t, err)
	got := stat.PGresult{}
	assert.NoError(t, json.Unmarshal(data, &got))                                    // unmarshal to JSON
	assert.Equal(t, stats, map[string]stat.PGresult{"pgcenter_record_testing": got}) // compare unmarshalled with origin

	// Cleanup.
	assert.NoError(t, os.Remove(filename))
}