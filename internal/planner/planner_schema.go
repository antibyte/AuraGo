package planner

import (
	"database/sql"
	"fmt"
	"strings"

	"aurago/internal/dbutil"
)

const plannerSchemaVersion = 5

func initPlannerSchema(db *sql.DB) error {
	version, err := dbutil.GetUserVersion(db)
	if err != nil {
		return fmt.Errorf("get planner schema version: %w", err)
	}
	hasAppointments, err := plannerTableExists(db, "appointments")
	if err != nil {
		return err
	}
	hasTodos, err := plannerTableExists(db, "todos")
	if err != nil {
		return err
	}

	switch {
	case !hasAppointments && !hasTodos:
		if err := createPlannerTables(db); err != nil {
			return err
		}
	case version < 2:
		if err := migratePlannerToV2(db, hasAppointments, hasTodos); err != nil {
			return err
		}
		fallthrough
	case version < 3:
		if err := migratePlannerToV3(db); err != nil {
			return err
		}
		fallthrough
	case version < 4:
		if err := migratePlannerToV4(db); err != nil {
			return err
		}
		fallthrough
	case version < plannerSchemaVersion:
		if err := migratePlannerToV5(db); err != nil {
			return err
		}
	default:
		if err := ensurePlannerIndexes(db); err != nil {
			return err
		}
	}

	if err := dbutil.SetUserVersion(db, plannerSchemaVersion); err != nil {
		return fmt.Errorf("set planner schema version: %w", err)
	}
	return nil
}

func plannerTableExists(db *sql.DB, table string) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check planner table %s: %w", table, err)
	}
	return count > 0, nil
}

func plannerColumnExists(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, fmt.Errorf("check planner column %s.%s: %w", table, column, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return false, fmt.Errorf("scan planner column %s.%s: %w", table, column, err)
		}
		if name == column {
			return true, nil
		}
	}
	return false, nil
}

func createPlannerTables(db *sql.DB) error {
	if _, err := db.Exec(plannerTablesSQL()); err != nil {
		return fmt.Errorf("create planner schema: %w", err)
	}
	return nil
}

func ensurePlannerIndexes(db *sql.DB) error {
	if _, err := db.Exec(plannerIndexesSQL()); err != nil {
		return fmt.Errorf("ensure planner indexes: %w", err)
	}
	return nil
}

func migratePlannerToV2(db *sql.DB, hasAppointments, hasTodos bool) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin planner migration: %w", err)
	}
	defer tx.Rollback()

	if hasAppointments {
		if _, err := tx.Exec(`ALTER TABLE appointments RENAME TO appointments_legacy`); err != nil {
			return fmt.Errorf("rename appointments table: %w", err)
		}
	}
	if hasTodos {
		if _, err := tx.Exec(`ALTER TABLE todos RENAME TO todos_legacy`); err != nil {
			return fmt.Errorf("rename todos table: %w", err)
		}
	}

	if _, err := tx.Exec(plannerTablesSQL()); err != nil {
		return fmt.Errorf("create migrated planner schema: %w", err)
	}

	if hasAppointments {
		if _, err := tx.Exec(`
			INSERT INTO appointments (id, title, description, date_time, notification_at, wake_agent, agent_instruction, notified, status, kg_node_id, created_at, updated_at)
			SELECT
				id,
				COALESCE(title, ''),
				COALESCE(description, ''),
				COALESCE(date_time, ''),
				COALESCE(notification_at, ''),
				CASE WHEN wake_agent IS NULL OR wake_agent = 0 THEN 0 ELSE 1 END,
				COALESCE(agent_instruction, ''),
				CASE WHEN notified IS NULL OR notified = 0 THEN 0 ELSE 1 END,
				CASE WHEN status IN ('upcoming', 'completed', 'cancelled') THEN status ELSE 'upcoming' END,
				COALESCE(kg_node_id, ''),
				COALESCE(created_at, ''),
				COALESCE(updated_at, '')
			FROM appointments_legacy
		`); err != nil {
			return fmt.Errorf("migrate appointments data: %w", err)
		}
		if _, err := tx.Exec(`DROP TABLE appointments_legacy`); err != nil {
			return fmt.Errorf("drop legacy appointments table: %w", err)
		}
	}

	if hasTodos {
		if _, err := tx.Exec(`
			INSERT INTO todos (id, title, description, priority, status, due_date, kg_node_id, created_at, updated_at)
			SELECT
				id,
				COALESCE(title, ''),
				COALESCE(description, ''),
				CASE WHEN priority IN ('low', 'medium', 'high') THEN priority ELSE 'medium' END,
				CASE WHEN status IN ('open', 'in_progress', 'done') THEN status ELSE 'open' END,
				COALESCE(due_date, ''),
				COALESCE(kg_node_id, ''),
				COALESCE(created_at, ''),
				COALESCE(updated_at, '')
			FROM todos_legacy
		`); err != nil {
			return fmt.Errorf("migrate todos data: %w", err)
		}
		if _, err := tx.Exec(`DROP TABLE todos_legacy`); err != nil {
			return fmt.Errorf("drop legacy todos table: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit planner migration: %w", err)
	}
	return nil
}

func migratePlannerToV3(db *sql.DB) error {
	hasAppointments, err := plannerTableExists(db, "appointments")
	if err != nil {
		return err
	}
	hasTodos, err := plannerTableExists(db, "todos")
	if err != nil {
		return err
	}
	if !hasAppointments {
		if _, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS appointments (
				id TEXT PRIMARY KEY,
				title TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '',
				date_time TEXT NOT NULL DEFAULT '',
				notification_at TEXT NOT NULL DEFAULT '',
				wake_agent INTEGER NOT NULL DEFAULT 0,
				agent_instruction TEXT NOT NULL DEFAULT '',
				notified INTEGER NOT NULL DEFAULT 0,
				status TEXT NOT NULL DEFAULT 'upcoming' CHECK (status IN ('upcoming', 'completed', 'cancelled')),
				kg_node_id TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			)`); err != nil {
			return fmt.Errorf("create appointments table: %w", err)
		}
	}
	if !hasTodos {
		if _, err := db.Exec(plannerTablesSQL()); err != nil {
			return fmt.Errorf("create planner schema v3: %w", err)
		}
		return nil
	}

	addColumnIfMissing := func(column, ddl string) error {
		exists, err := plannerColumnExists(db, "todos", column)
		if err != nil {
			return err
		}
		if exists {
			return nil
		}
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("add todos.%s: %w", column, err)
		}
		return nil
	}

	if err := addColumnIfMissing("remind_daily", `ALTER TABLE todos ADD COLUMN remind_daily INTEGER NOT NULL DEFAULT 0`); err != nil {
		return err
	}
	if err := addColumnIfMissing("completed_at", `ALTER TABLE todos ADD COLUMN completed_at TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if err := addColumnIfMissing("last_daily_reminder_at", `ALTER TABLE todos ADD COLUMN last_daily_reminder_at TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE todos SET completed_at = COALESCE(updated_at, '') WHERE status = 'done' AND completed_at = ''`); err != nil {
		return fmt.Errorf("backfill todos.completed_at: %w", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS todo_items (
			id TEXT PRIMARY KEY,
			todo_id TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			position INTEGER NOT NULL DEFAULT 0,
			is_done INTEGER NOT NULL DEFAULT 0,
			completed_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY (todo_id) REFERENCES todos(id) ON DELETE CASCADE
		)`); err != nil {
		return fmt.Errorf("create todo_items table: %w", err)
	}
	return ensurePlannerIndexes(db)
}

func migratePlannerToV4(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS planner_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT ''
		)`); err != nil {
		return fmt.Errorf("create planner_meta table: %w", err)
	}
	return ensurePlannerIndexes(db)
}

func migratePlannerToV5(db *sql.DB) error {
	hasContacts, err := plannerTableExists(db, "appointment_contacts")
	if err != nil {
		return err
	}
	if !hasContacts {
		if _, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS appointment_contacts (
				appointment_id TEXT NOT NULL,
				contact_id TEXT NOT NULL,
				UNIQUE(appointment_id, contact_id)
			)`); err != nil {
			return fmt.Errorf("create appointment_contacts table: %w", err)
		}
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_appointment_contacts_appointment ON appointment_contacts(appointment_id)`); err != nil {
		return fmt.Errorf("create appointment_contacts appointment index: %w", err)
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_appointment_contacts_contact ON appointment_contacts(contact_id)`); err != nil {
		return fmt.Errorf("create appointment_contacts contact index: %w", err)
	}
	return nil
}

func plannerTablesSQL() string {
	return strings.TrimSpace(`
		CREATE TABLE IF NOT EXISTS appointments (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			date_time TEXT NOT NULL DEFAULT '',
			notification_at TEXT NOT NULL DEFAULT '',
			wake_agent INTEGER NOT NULL DEFAULT 0,
			agent_instruction TEXT NOT NULL DEFAULT '',
			notified INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'upcoming' CHECK (status IN ('upcoming', 'completed', 'cancelled')),
			kg_node_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS todos (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			priority TEXT NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high')),
			status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'in_progress', 'done')),
			due_date TEXT NOT NULL DEFAULT '',
			remind_daily INTEGER NOT NULL DEFAULT 0,
			completed_at TEXT NOT NULL DEFAULT '',
			last_daily_reminder_at TEXT NOT NULL DEFAULT '',
			kg_node_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS todo_items (
			id TEXT PRIMARY KEY,
			todo_id TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			position INTEGER NOT NULL DEFAULT 0,
			is_done INTEGER NOT NULL DEFAULT 0,
			completed_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY (todo_id) REFERENCES todos(id) ON DELETE CASCADE
		);
		CREATE TABLE IF NOT EXISTS planner_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT ''
		);
	` + plannerIndexesSQL())
}

func plannerIndexesSQL() string {
	return strings.TrimSpace(`
		CREATE INDEX IF NOT EXISTS idx_appointments_date ON appointments(date_time);
		CREATE INDEX IF NOT EXISTS idx_appointments_status ON appointments(status);
		CREATE INDEX IF NOT EXISTS idx_appointments_notification ON appointments(notification_at, notified);
		CREATE INDEX IF NOT EXISTS idx_todos_status ON todos(status);
		CREATE INDEX IF NOT EXISTS idx_todos_priority ON todos(priority);
		CREATE INDEX IF NOT EXISTS idx_todos_due ON todos(due_date);
		CREATE INDEX IF NOT EXISTS idx_todos_remind_daily ON todos(remind_daily, status);
		CREATE INDEX IF NOT EXISTS idx_todo_items_todo ON todo_items(todo_id);
		CREATE INDEX IF NOT EXISTS idx_todo_items_order ON todo_items(todo_id, position);
	`)
}
