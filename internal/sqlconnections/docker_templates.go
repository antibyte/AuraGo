package sqlconnections

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// DockerTemplate describes how to create a database container.
type DockerTemplate struct {
	Driver       string            // "postgres" or "mysql"
	Image        string            // e.g. "postgres:16"
	DefaultPort  int               // container internal port
	EnvVars      map[string]string // environment variables; PASSWORD_PLACEHOLDER is replaced at creation
	VolumeSuffix string            // appended to "aurago_db_<name>_" for data volume
}

// DockerDBRequest is the result of applying a template — ready for DockerCreateContainer.
type DockerDBRequest struct {
	ContainerName string
	Image         string
	Env           []string
	Ports         map[string]string
	Volumes       []string
	Restart       string
	// Derived connection info
	Driver       string
	Host         string
	Port         int
	DatabaseName string
	Username     string
	Password     string
}

var templates = map[string]DockerTemplate{
	"postgres": {
		Driver:      "postgres",
		Image:       "postgres:16-alpine",
		DefaultPort: 5432,
		EnvVars: map[string]string{
			"POSTGRES_USER":     "aurago",
			"POSTGRES_PASSWORD": "PASSWORD_PLACEHOLDER",
			"POSTGRES_DB":       "DB_PLACEHOLDER",
		},
		VolumeSuffix: "pgdata",
	},
	"mysql": {
		Driver:      "mysql",
		Image:       "mysql:8.0",
		DefaultPort: 3306,
		EnvVars: map[string]string{
			"MYSQL_USER":          "aurago",
			"MYSQL_PASSWORD":      "PASSWORD_PLACEHOLDER",
			"MYSQL_ROOT_PASSWORD": "PASSWORD_PLACEHOLDER",
			"MYSQL_DATABASE":      "DB_PLACEHOLDER",
		},
		VolumeSuffix: "mysqldata",
	},
	"mariadb": {
		Driver:      "mysql",
		Image:       "mariadb:11",
		DefaultPort: 3306,
		EnvVars: map[string]string{
			"MARIADB_USER":          "aurago",
			"MARIADB_PASSWORD":      "PASSWORD_PLACEHOLDER",
			"MARIADB_ROOT_PASSWORD": "PASSWORD_PLACEHOLDER",
			"MARIADB_DATABASE":      "DB_PLACEHOLDER",
		},
		VolumeSuffix: "mariadbdata",
	},
}

// ListTemplates returns the available template names.
func ListTemplates() []string {
	return []string{"postgres", "mysql", "mariadb"}
}

// PrepareDockerDB applies a template and generates a ready-to-create container request.
func PrepareDockerDB(templateName, connectionName, databaseName string) (*DockerDBRequest, error) {
	tmpl, ok := templates[templateName]
	if !ok {
		return nil, fmt.Errorf("unknown docker template: %s (available: postgres, mysql, mariadb)", templateName)
	}

	if connectionName == "" {
		return nil, fmt.Errorf("connection name is required")
	}
	if databaseName == "" {
		databaseName = connectionName
	}

	password, err := generatePassword(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate password: %w", err)
	}

	containerName := fmt.Sprintf("aurago-db-%s", connectionName)
	volumeName := fmt.Sprintf("aurago_db_%s_%s", connectionName, tmpl.VolumeSuffix)
	hostPort := fmt.Sprintf("%d", tmpl.DefaultPort)

	env := make([]string, 0, len(tmpl.EnvVars))
	for k, v := range tmpl.EnvVars {
		switch v {
		case "PASSWORD_PLACEHOLDER":
			v = password
		case "DB_PLACEHOLDER":
			v = databaseName
		}
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return &DockerDBRequest{
		ContainerName: containerName,
		Image:         tmpl.Image,
		Env:           env,
		Ports:         map[string]string{hostPort: hostPort},
		Volumes:       []string{fmt.Sprintf("%s:/var/lib/%s", volumeName, tmpl.VolumeSuffix)},
		Restart:       "unless-stopped",
		Driver:        tmpl.Driver,
		Host:          "localhost",
		Port:          tmpl.DefaultPort,
		DatabaseName:  databaseName,
		Username:      "aurago",
		Password:      password,
	}, nil
}

func generatePassword(length int) (string, error) {
	b := make([]byte, length/2)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
