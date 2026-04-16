package sqlconnections

import "testing"

func TestPrepareDockerDBUsesOfficialDataPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		templateName string
		connection   string
		database     string
		wantVolume   string
	}{
		{
			name:         "postgres uses postgres data directory",
			templateName: "postgres",
			connection:   "app",
			database:     "appdb",
			wantVolume:   "aurago_db_app_pgdata:/var/lib/postgresql/data",
		},
		{
			name:         "mysql uses mysql data directory",
			templateName: "mysql",
			connection:   "shop",
			database:     "shopdb",
			wantVolume:   "aurago_db_shop_mysqldata:/var/lib/mysql",
		},
		{
			name:         "mariadb uses mysql data directory",
			templateName: "mariadb",
			connection:   "crm",
			database:     "crmdb",
			wantVolume:   "aurago_db_crm_mariadbdata:/var/lib/mysql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req, err := PrepareDockerDB(tt.templateName, tt.connection, tt.database)
			if err != nil {
				t.Fatalf("PrepareDockerDB() error = %v", err)
			}

			if len(req.Volumes) != 1 {
				t.Fatalf("len(Volumes) = %d, want 1", len(req.Volumes))
			}
			if req.Volumes[0] != tt.wantVolume {
				t.Fatalf("volume = %q, want %q", req.Volumes[0], tt.wantVolume)
			}
		})
	}
}
