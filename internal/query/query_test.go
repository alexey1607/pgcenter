package query

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFormat(t *testing.T) {
	opts := Options{
		WalFunction1: "pg_wal_lsn_diff",
		WalFunction2: "pg_current_wal_lsn",
	}
	got, err := Format(PgStatReplicationDefault, opts)
	assert.NoError(t, err)
	assert.Equal(
		t,
		"SELECT pid AS pid, client_addr AS client, usename AS user, application_name AS name, state, sync_state AS mode, (pg_wal_lsn_diff(pg_current_wal_lsn(),'0/0') / 1024)::bigint AS wal, (pg_wal_lsn_diff(pg_current_wal_lsn(),sent_lsn) / 1024)::bigint AS pending, (pg_wal_lsn_diff(sent_lsn,write_lsn) / 1024)::bigint AS write, (pg_wal_lsn_diff(write_lsn,flush_lsn) / 1024)::bigint AS flush, (pg_wal_lsn_diff(flush_lsn,replay_lsn) / 1024)::bigint AS replay, (pg_wal_lsn_diff(pg_current_wal_lsn(),replay_lsn))::bigint / 1024 AS total_lag, coalesce(date_trunc('seconds', write_lag), '0 seconds'::interval) AS write_lag, coalesce(date_trunc('seconds', flush_lag), '0 seconds'::interval) AS flush_lag, coalesce(date_trunc('seconds', replay_lag), '0 seconds'::interval) AS replay_lag FROM pg_stat_replication ORDER BY pid DESC",
		got,
	)

	_, err = Format("{{ .Invalid }}", opts)
	assert.Error(t, err)
}

func TestOptions_Configure(t *testing.T) {
	testcases := []struct {
		version  int
		recovery string
		program  string
		want     Options
	}{
		{version: 130000, recovery: "f", program: "top", want: Options{
			ViewType: "user", WalFunction1: "pg_wal_lsn_diff", WalFunction2: "pg_current_wal_lsn",
			QueryAgeThresh: "00:00:00.0", ShowNoIdle: true, PgSSQueryLenFn: "left(p.query, 256)",
		}},
		{version: 130000, recovery: "t", program: "top", want: Options{
			ViewType: "user", WalFunction1: "pg_wal_lsn_diff", WalFunction2: "pg_last_wal_receive_lsn",
			QueryAgeThresh: "00:00:00.0", ShowNoIdle: true, PgSSQueryLenFn: "left(p.query, 256)",
		}},
		{version: 96000, recovery: "f", program: "top", want: Options{
			ViewType: "user", WalFunction1: "pg_xlog_location_diff", WalFunction2: "pg_current_xlog_location",
			QueryAgeThresh: "00:00:00.0", ShowNoIdle: true, PgSSQueryLenFn: "left(p.query, 256)",
		}},
		{version: 96000, recovery: "t", program: "top", want: Options{
			ViewType: "user", WalFunction1: "pg_xlog_location_diff", WalFunction2: "pg_last_xlog_receive_location",
			QueryAgeThresh: "00:00:00.0", ShowNoIdle: true, PgSSQueryLenFn: "left(p.query, 256)",
		}},
		{version: 130000, recovery: "f", program: "record", want: Options{
			ViewType: "user", WalFunction1: "pg_wal_lsn_diff", WalFunction2: "pg_current_wal_lsn",
			QueryAgeThresh: "00:00:00.0", ShowNoIdle: true, PgSSQueryLen: 0, PgSSQueryLenFn: "p.query",
		}},
	}

	for _, tc := range testcases {
		opts := Options{}
		opts.Configure(tc.version, tc.recovery, tc.program)
		assert.Equal(t, tc.want, opts)
	}

	opts := Options{PgSSQueryLen: 123}
	opts.Configure(130000, "f", "record")
	assert.Equal(
		t, Options{
			ViewType: "user", WalFunction1: "pg_wal_lsn_diff", WalFunction2: "pg_current_wal_lsn",
			QueryAgeThresh: "00:00:00.0", ShowNoIdle: true, PgSSQueryLen: 123, PgSSQueryLenFn: "left(p.query, 123)",
		},
		opts,
	)
}