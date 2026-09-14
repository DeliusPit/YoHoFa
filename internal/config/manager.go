package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	drivermysql "github.com/go-sql-driver/mysql"
)

var (
	ErrReadOnly = errors.New("configuration is read-only")
	ErrInvalid  = errors.New("invalid storage configuration")
)

type DatabaseInput struct {
	Backend  string         `json:"backend"`
	SQLite   *SQLiteInput   `json:"sqlite,omitempty"`
	Postgres *PostgresInput `json:"postgres,omitempty"`
	MySQL    *MySQLInput    `json:"mysql,omitempty"`
	Host     string         `json:"host,omitempty"`
	Port     int            `json:"port,omitempty"`
	Name     string         `json:"name,omitempty"`
	User     string         `json:"user,omitempty"`
	SSLMode  string         `json:"sslMode,omitempty"`
	Password *string        `json:"password,omitempty"`
}

type SQLiteInput struct {
	Path string `json:"path"`
}

type PostgresInput struct {
	Host     string  `json:"host"`
	Port     int     `json:"port"`
	Name     string  `json:"name"`
	User     string  `json:"user"`
	TLSMode  string  `json:"tlsMode"`
	Password *string `json:"password,omitempty"`
}

type MySQLInput struct {
	Host     string  `json:"host"`
	Port     int     `json:"port"`
	Name     string  `json:"name"`
	User     string  `json:"user"`
	TLSMode  string  `json:"tlsMode"`
	Password *string `json:"password,omitempty"`
}

type ObjectInput struct {
	Backend      string  `json:"backend"`
	Path         string  `json:"path"`
	Endpoint     string  `json:"endpoint"`
	AccessKey    *string `json:"accessKey,omitempty"`
	SecretKey    *string `json:"secretKey,omitempty"`
	Bucket       string  `json:"bucket"`
	UseTLS       bool    `json:"useTLS"`
	Region       string  `json:"region"`
	PathStyle    bool    `json:"pathStyle"`
	CreateBucket bool    `json:"createBucket"`
}

type Update struct {
	Profile  string        `json:"profile"`
	Database DatabaseInput `json:"database"`
	Objects  ObjectInput   `json:"objects"`
}

type DatabaseView struct {
	Backend     string       `json:"backend"`
	Host        string       `json:"host"`
	Port        int          `json:"port"`
	Name        string       `json:"name"`
	User        string       `json:"user"`
	SSLMode     string       `json:"sslMode"`
	PasswordSet bool         `json:"passwordSet"`
	SQLite      SQLiteView   `json:"sqlite"`
	Postgres    PostgresView `json:"postgres"`
	MySQL       MySQLView    `json:"mysql"`
}

type SQLiteView struct {
	Path string `json:"path"`
}

type PostgresView struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Name        string `json:"name"`
	User        string `json:"user"`
	TLSMode     string `json:"tlsMode"`
	PasswordSet bool   `json:"passwordSet"`
}

type MySQLView struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Name        string `json:"name"`
	User        string `json:"user"`
	TLSMode     string `json:"tlsMode"`
	PasswordSet bool   `json:"passwordSet"`
}

type ObjectView struct {
	Backend      string `json:"backend"`
	Path         string `json:"path"`
	Endpoint     string `json:"endpoint"`
	AccessKeySet bool   `json:"accessKeySet"`
	SecretKeySet bool   `json:"secretKeySet"`
	Bucket       string `json:"bucket"`
	UseTLS       bool   `json:"useTLS"`
	Region       string `json:"region"`
	PathStyle    bool   `json:"pathStyle"`
	CreateBucket bool   `json:"createBucket"`
}

type SettingsView struct {
	Profile  string       `json:"profile"`
	Database DatabaseView `json:"database"`
	Objects  ObjectView   `json:"objects"`
}

type Capability struct {
	Value     string `json:"value"`
	Supported bool   `json:"supported"`
	Label     string `json:"label,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type Capabilities struct {
	Profiles     []Capability `json:"profiles"`
	Databases    []Capability `json:"databases"`
	ObjectStores []Capability `json:"objectStores"`
}

type View struct {
	SettingsView
	Source          string        `json:"source"`
	Editable        bool          `json:"editable"`
	ReadOnlyReason  string        `json:"readOnlyReason,omitempty"`
	RestartRequired bool          `json:"restartRequired"`
	Effective       SettingsView  `json:"effective"`
	Pending         *SettingsView `json:"pending,omitempty"`
	Capabilities    Capabilities  `json:"capabilities"`
}

type diskConfig struct {
	Version        int    `json:"version,omitempty"`
	StorageProfile string `json:"storageProfile"`
	Database       struct {
		Backend string `json:"backend"`
		URL     string `json:"url,omitempty"`
		SQLite  struct {
			Path string `json:"path"`
		} `json:"sqlite,omitempty"`
		Postgres struct {
			URL string `json:"url"`
		} `json:"postgres,omitempty"`
		MySQL struct {
			Host     string `json:"host"`
			Port     int    `json:"port"`
			Name     string `json:"name"`
			User     string `json:"user"`
			Password string `json:"password"`
			TLSMode  string `json:"tlsMode"`
		} `json:"mysql,omitempty"`
	} `json:"database"`
	Object struct {
		Backend    string `json:"backend"`
		Filesystem struct {
			Path string `json:"path"`
		} `json:"filesystem"`
		S3 struct {
			Endpoint     string `json:"endpoint"`
			AccessKey    string `json:"accessKey"`
			SecretKey    string `json:"secretKey"`
			Bucket       string `json:"bucket"`
			UseTLS       bool   `json:"useTLS"`
			Region       string `json:"region"`
			PathStyle    bool   `json:"pathStyle"`
			CreateBucket bool   `json:"createBucket"`
		} `json:"s3"`
	} `json:"object"`
}

type Manager struct {
	mu             sync.RWMutex
	path           string
	source         string
	editable       bool
	readOnlyReason string
	effective      Config
	pending        *Config
}

func NewManager(environment Config) (*Manager, error) {
	m := &Manager{path: environment.ConfigPath, source: "environment", effective: environment}
	data, exists, err := readConfigFile(m.path)
	if err != nil {
		return nil, err
	}
	if exists {
		var persisted diskConfig
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&persisted); err != nil {
			return nil, fmt.Errorf("read configuration file: invalid configuration")
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("read configuration file: invalid configuration")
		}
		if persisted.Version != 0 && persisted.Version != 2 {
			return nil, fmt.Errorf("read configuration file: unsupported configuration version")
		}
		loaded := persisted.apply(environment)
		if persisted.Version == 2 {
			loaded = normalizeV2DatabaseConfig(loaded)
			if err := validateRetainedDatabases(loaded); err != nil {
				return nil, err
			}
			if mysqlConfigured(loaded) {
				loaded.MySQLDSN = buildMySQLDSN(loaded)
			}
		}
		if err := validate(loaded); err != nil {
			return nil, err
		}
		m.source = "file"
		m.effective = loaded
	} else if environment.ConfigBootstrapFromEnv {
		if err := validate(environment); err != nil {
			return nil, fmt.Errorf("bootstrap configuration: %w", err)
		}
		if err := writeAtomic(m.path, diskFromConfig(environment)); err != nil {
			return nil, fmt.Errorf("bootstrap configuration: %w", err)
		}
		m.source = "file"
	} else if storageEnvironmentConfigured() {
		if err := validate(environment); err != nil {
			return nil, err
		}
		m.readOnlyReason = "Systeemconfiguratie is alleen-lezen omdat opslaginstellingen uit environment variables komen."
		return m, nil
	}
	if !environment.ConfigEditable {
		m.readOnlyReason = "Systeemconfiguratie is alleen-lezen omdat CONFIG_EDITABLE niet is ingeschakeld."
		return m, nil
	}
	if !authConfigured(environment) && !developmentLoopback(environment) {
		m.readOnlyReason = "Systeemconfiguratie is alleen-lezen: stel AUTH_USERNAME en AUTH_PASSWORD volledig in."
		return m, nil
	}
	m.source = "file"
	m.editable = true
	return m, nil
}

func (m *Manager) Effective() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.effective
}

func (m *Manager) View() View {
	m.mu.RLock()
	defer m.mu.RUnlock()
	effective := safeView(m.effective)
	displayed := effective
	view := View{
		SettingsView:    displayed,
		Source:          m.source,
		Editable:        m.editable,
		ReadOnlyReason:  m.readOnlyReason,
		RestartRequired: m.pending != nil,
		Effective:       effective,
		Capabilities: Capabilities{
			Profiles:     []Capability{{Value: "local", Supported: true, Label: "Lokale computer"}, {Value: "server", Supported: true}, {Value: "central", Supported: true}},
			Databases:    []Capability{{Value: "postgres", Supported: true}, {Value: "sqlite", Supported: true, Label: "SQLite"}, {Value: "mysql", Supported: true, Label: "MySQL"}},
			ObjectStores: []Capability{{Value: "s3", Supported: true}, {Value: "filesystem", Supported: true}},
		},
	}
	if m.pending != nil {
		pending := safeView(*m.pending)
		view.Pending = &pending
		view.SettingsView = pending
	}
	return view
}

func (m *Manager) Save(input Update) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.editable {
		return ErrReadOnly
	}

	base := m.effective
	if m.pending != nil {
		base = *m.pending
	}
	next, err := input.config(base)
	if err != nil {
		return err
	}
	next = normalizeV2DatabaseConfig(next)
	if err := validateRetainedDatabases(next); err != nil {
		return err
	}
	if mysqlConfigured(next) {
		next.MySQLDSN = buildMySQLDSN(next)
	}
	if err := validate(next); err != nil {
		return err
	}
	if err := writeAtomic(m.path, diskFromConfig(next)); err != nil {
		return fmt.Errorf("persist configuration: %w", err)
	}
	m.pending = &next
	return nil
}

func (input Update) config(base Config) (Config, error) {
	next := normalizeDatabaseConfig(base)
	next.StorageProfile = strings.TrimSpace(input.Profile)
	next.StoreBackend = strings.TrimSpace(input.Database.Backend)
	next.ObjectBackend = strings.TrimSpace(input.Objects.Backend)
	if path := strings.TrimSpace(input.Objects.Path); next.ObjectBackend == "filesystem" || path != "" {
		next.ObjectFilesystemRoot = path
	}
	if next.ObjectBackend == "s3" {
		next.S3Endpoint = strings.TrimSpace(input.Objects.Endpoint)
		next.S3Bucket = strings.TrimSpace(input.Objects.Bucket)
		next.S3UseTLS = input.Objects.UseTLS
		next.S3Region = strings.TrimSpace(input.Objects.Region)
		next.S3PathStyle = input.Objects.PathStyle
		next.S3CreateBucket = input.Objects.CreateBucket
	}
	postgres := input.Database.Postgres
	if postgres == nil && next.StoreBackend == "postgres" {
		postgres = &PostgresInput{Host: input.Database.Host, Port: input.Database.Port, Name: input.Database.Name, User: input.Database.User, TLSMode: input.Database.SSLMode, Password: input.Database.Password}
	}
	if postgres != nil {
		password := databasePassword(next.PostgresURL)
		if postgres.Password != nil {
			password = *postgres.Password
		}
		next.PostgresURL = buildPostgresURL(*postgres, password, next.PostgresURL)
		if err := validatePostgresURL(next.PostgresURL); err != nil {
			return Config{}, err
		}
	}
	if input.Database.SQLite != nil {
		next.DatabasePath = strings.TrimSpace(input.Database.SQLite.Path)
		if err := validateSQLitePath(next.DatabasePath, next.DatabaseDataRoot); err != nil {
			return Config{}, err
		}
	}
	if input.Database.MySQL != nil {
		mysql := input.Database.MySQL
		next.MySQLHost = strings.TrimSpace(mysql.Host)
		next.MySQLPort = mysql.Port
		if next.MySQLPort == 0 {
			next.MySQLPort = 3306
		}
		next.MySQLName = strings.TrimSpace(mysql.Name)
		next.MySQLUser = strings.TrimSpace(mysql.User)
		next.MySQLTLSMode = normalizeMySQLTLSMode(mysql.TLSMode)
		if mysql.Password != nil {
			next.MySQLPassword = *mysql.Password
		}
		if err := validateMySQL(next); err != nil {
			return Config{}, err
		}
		next.MySQLDSN = buildMySQLDSN(next)
	}
	next.DatabaseURL = next.PostgresURL
	if input.Objects.AccessKey != nil {
		next.S3AccessKey = *input.Objects.AccessKey
	}
	if input.Objects.SecretKey != nil {
		next.S3SecretKey = *input.Objects.SecretKey
	}
	return next, nil
}

func validate(cfg Config) error {
	if err := validateCombination(cfg); err != nil {
		return ErrInvalid
	}
	return validateObjectStore(cfg)
}

func validateCombination(cfg Config) error {
	switch cfg.StorageProfile {
	case "local":
		if cfg.StoreBackend != "sqlite" || cfg.ObjectBackend != "filesystem" {
			return ErrInvalid
		}
		if err := validateSQLitePath(cfg.DatabasePath, cfg.DatabaseDataRoot); err != nil {
			return err
		}
	case "server", "central":
		if cfg.StoreBackend != "postgres" && cfg.StoreBackend != "mysql" {
			return ErrInvalid
		}
		if cfg.StoreBackend == "postgres" {
			if err := validatePostgresURL(cfg.DatabaseURL); err != nil {
				return err
			}
		} else if err := validateMySQL(cfg); err != nil {
			return err
		}
	default:
		return ErrInvalid
	}
	if cfg.ObjectBackend != "filesystem" && cfg.ObjectBackend != "s3" {
		return ErrInvalid
	}
	return nil
}

func validateObjectStore(cfg Config) error {
	if cfg.ObjectBackend == "filesystem" {
		cleaned := filepath.Clean(cfg.ObjectFilesystemRoot)
		if !filepath.IsAbs(cfg.ObjectFilesystemRoot) || cleaned == string(filepath.Separator) {
			return ErrInvalid
		}
		return nil
	}
	if cfg.ObjectBackend != "s3" || cfg.S3Endpoint == "" || cfg.S3Bucket == "" || cfg.S3Region == "" || cfg.S3AccessKey == "" || cfg.S3SecretKey == "" {
		return ErrInvalid
	}
	if strings.Contains(cfg.S3Endpoint, "://") || strings.ContainsAny(cfg.S3Endpoint, "/?#") {
		return ErrInvalid
	}
	endpoint, err := url.Parse("http://" + cfg.S3Endpoint)
	if err != nil || endpoint.Host != cfg.S3Endpoint || endpoint.Hostname() == "" || endpoint.User != nil {
		return ErrInvalid
	}
	if port := endpoint.Port(); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return ErrInvalid
		}
	}
	return nil
}

func safeView(cfg Config) SettingsView {
	cfg = normalizeDatabaseConfig(cfg)
	parsed, _ := url.Parse(cfg.PostgresURL)
	port, _ := strconv.Atoi(parsed.Port())
	if port == 0 {
		port = 5432
	}
	username := ""
	passwordSet := false
	if parsed.User != nil {
		username = parsed.User.Username()
		password, _ := parsed.User.Password()
		passwordSet = password != ""
	}
	postgres := PostgresView{Host: parsed.Hostname(), Port: port, Name: strings.TrimPrefix(parsed.Path, "/"), User: username, TLSMode: parsed.Query().Get("sslmode"), PasswordSet: passwordSet}
	mysqlPort := cfg.MySQLPort
	if mysqlPort == 0 {
		mysqlPort = 3306
	}
	mysqlTLS := cfg.MySQLTLSMode
	if mysqlTLS == "" {
		mysqlTLS = "required"
	}
	database := DatabaseView{
		Backend: cfg.StoreBackend, SQLite: SQLiteView{Path: cfg.DatabasePath}, Postgres: postgres,
		MySQL: MySQLView{Host: cfg.MySQLHost, Port: mysqlPort, Name: cfg.MySQLName, User: cfg.MySQLUser, TLSMode: mysqlTLS, PasswordSet: cfg.MySQLPassword != ""},
	}
	// Flat fields keep older frontends usable during a rolling deployment.
	if cfg.StoreBackend == "sqlite" {
		database.Name = cfg.DatabasePath
	} else if cfg.StoreBackend == "mysql" {
		database.Host, database.Port, database.Name, database.User, database.SSLMode, database.PasswordSet = database.MySQL.Host, database.MySQL.Port, database.MySQL.Name, database.MySQL.User, database.MySQL.TLSMode, database.MySQL.PasswordSet
	} else {
		database.Host, database.Port, database.Name, database.User, database.SSLMode, database.PasswordSet = postgres.Host, postgres.Port, postgres.Name, postgres.User, postgres.TLSMode, postgres.PasswordSet
	}
	return SettingsView{
		Profile:  cfg.StorageProfile,
		Database: database,
		Objects:  ObjectView{Backend: cfg.ObjectBackend, Path: cfg.ObjectFilesystemRoot, Endpoint: cfg.S3Endpoint, AccessKeySet: cfg.S3AccessKey != "", SecretKeySet: cfg.S3SecretKey != "", Bucket: cfg.S3Bucket, UseTLS: cfg.S3UseTLS, Region: cfg.S3Region, PathStyle: cfg.S3PathStyle, CreateBucket: cfg.S3CreateBucket},
	}
}

func buildPostgresURL(input PostgresInput, password string, baseURL string) string {
	user := url.UserPassword(strings.TrimSpace(input.User), password)
	port := input.Port
	if port == 0 {
		port = 5432
	}
	host := net.JoinHostPort(strings.TrimSpace(input.Host), strconv.Itoa(port))
	result := &url.URL{Scheme: "postgres", User: user, Host: host, Path: "/" + strings.TrimPrefix(strings.TrimSpace(input.Name), "/")}
	base, _ := url.Parse(baseURL)
	query := base.Query()
	query.Set("sslmode", strings.TrimSpace(input.TLSMode))
	result.RawQuery = query.Encode()
	return result.String()
}

func normalizeDatabaseConfig(cfg Config) Config {
	if cfg.PostgresURL == "" {
		cfg.PostgresURL = cfg.DatabaseURL
	}
	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = cfg.PostgresURL
	}
	if cfg.MySQLPort == 0 {
		cfg.MySQLPort = 3306
	}
	if cfg.MySQLTLSMode == "" {
		cfg.MySQLTLSMode = "required"
	} else if cfg.MySQLTLSMode == "preferred" {
		cfg.MySQLTLSMode = "required"
	}
	return cfg
}

func normalizeV2DatabaseConfig(cfg Config) Config {
	cfg = normalizeDatabaseConfig(cfg)
	if cfg.StoreBackend != "sqlite" && cfg.DatabasePath != "" {
		if err := validateSQLitePath(cfg.DatabasePath, cfg.DatabaseDataRoot); err != nil {
			cfg.DatabasePath = ""
		}
	}
	return cfg
}

func validateRetainedDatabases(cfg Config) error {
	if err := validatePostgresURL(cfg.PostgresURL); err != nil {
		return err
	}
	if cfg.DatabasePath != "" {
		if err := validateSQLitePath(cfg.DatabasePath, cfg.DatabaseDataRoot); err != nil {
			return err
		}
	}
	if mysqlConfigured(cfg) {
		return validateMySQL(cfg)
	}
	return nil
}

func validatePostgresURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Hostname() == "" || parsed.User == nil || parsed.User.Username() == "" || strings.TrimPrefix(parsed.Path, "/") == "" {
		return ErrInvalid
	}
	if port := parsed.Port(); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return ErrInvalid
		}
	}
	sslmode := parsed.Query().Get("sslmode")
	if sslmode != "disable" && sslmode != "require" && sslmode != "verify-ca" && sslmode != "verify-full" {
		return ErrInvalid
	}
	return nil
}

func validateSQLitePath(path, dataRoot string) error {
	cleaned := filepath.Clean(path)
	if path == "" || !filepath.IsAbs(path) || cleaned == string(filepath.Separator) || strings.Contains(path, "://") || strings.ContainsAny(path, "?#\x00") || strings.HasPrefix(strings.ToLower(path), "file:") {
		return ErrInvalid
	}
	if dataRoot == "" {
		return nil
	}
	root := filepath.Clean(dataRoot)
	if !filepath.IsAbs(root) || root == string(filepath.Separator) {
		return ErrInvalid
	}
	relative, err := filepath.Rel(root, cleaned)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return ErrInvalid
	}
	return nil
}

func validateMySQL(cfg Config) error {
	if cfg.MySQLHost == "" || cfg.MySQLName == "" || cfg.MySQLUser == "" || cfg.MySQLPort < 1 || cfg.MySQLPort > 65535 {
		return ErrInvalid
	}
	if strings.ContainsAny(cfg.MySQLHost, "/\\@?#(), \t\r\n") || strings.Contains(cfg.MySQLUser, ":") || containsControl(cfg.MySQLHost) || containsControl(cfg.MySQLUser) || containsControl(cfg.MySQLName) || containsControl(cfg.MySQLPassword) {
		return ErrInvalid
	}
	if strings.Contains(cfg.MySQLHost, ":") && net.ParseIP(cfg.MySQLHost) == nil {
		return ErrInvalid
	}
	if cfg.MySQLTLSMode != "disabled" && cfg.MySQLTLSMode != "required" {
		return ErrInvalid
	}
	return nil
}

func buildMySQLDSN(cfg Config) string {
	port := cfg.MySQLPort
	if port == 0 {
		port = 3306
	}
	mode := cfg.MySQLTLSMode
	if mode == "" {
		mode = "required"
	}
	driverConfig := drivermysql.NewConfig()
	driverConfig.User = cfg.MySQLUser
	driverConfig.Passwd = cfg.MySQLPassword
	driverConfig.Net = "tcp"
	driverConfig.Addr = net.JoinHostPort(cfg.MySQLHost, strconv.Itoa(port))
	driverConfig.DBName = cfg.MySQLName
	driverConfig.TLSConfig = map[string]string{"disabled": "false", "required": "true"}[mode]
	return driverConfig.FormatDSN()
}

func normalizeMySQLTLSMode(mode string) string {
	mode = strings.TrimSpace(mode)
	if mode == "" || mode == "preferred" {
		return "required"
	}
	return mode
}

func mysqlConfigured(cfg Config) bool {
	return cfg.MySQLHost != "" || cfg.MySQLName != "" || cfg.MySQLUser != "" || cfg.MySQLPassword != ""
}

func containsControl(value string) bool {
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return true
		}
	}
	return false
}

func databasePassword(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.User == nil {
		return ""
	}
	password, _ := parsed.User.Password()
	return password
}

func diskFromConfig(cfg Config) diskConfig {
	cfg = normalizeDatabaseConfig(cfg)
	var persisted diskConfig
	persisted.Version = 2
	persisted.StorageProfile = cfg.StorageProfile
	persisted.Database.Backend = cfg.StoreBackend
	persisted.Database.SQLite.Path = cfg.DatabasePath
	persisted.Database.Postgres.URL = cfg.PostgresURL
	persisted.Database.MySQL.Host = cfg.MySQLHost
	persisted.Database.MySQL.Port = cfg.MySQLPort
	persisted.Database.MySQL.Name = cfg.MySQLName
	persisted.Database.MySQL.User = cfg.MySQLUser
	persisted.Database.MySQL.Password = cfg.MySQLPassword
	persisted.Database.MySQL.TLSMode = cfg.MySQLTLSMode
	persisted.Object.Backend = cfg.ObjectBackend
	persisted.Object.Filesystem.Path = cfg.ObjectFilesystemRoot
	persisted.Object.S3.Endpoint = cfg.S3Endpoint
	persisted.Object.S3.AccessKey = cfg.S3AccessKey
	persisted.Object.S3.SecretKey = cfg.S3SecretKey
	persisted.Object.S3.Bucket = cfg.S3Bucket
	persisted.Object.S3.UseTLS = cfg.S3UseTLS
	persisted.Object.S3.Region = cfg.S3Region
	persisted.Object.S3.PathStyle = cfg.S3PathStyle
	persisted.Object.S3.CreateBucket = cfg.S3CreateBucket
	return persisted
}

func (persisted diskConfig) apply(base Config) Config {
	base.StorageProfile = persisted.StorageProfile
	base.StoreBackend = persisted.Database.Backend
	if persisted.Version == 0 {
		base.DatabaseURL = persisted.Database.URL
		base.PostgresURL = persisted.Database.URL
	} else {
		base.DatabasePath = persisted.Database.SQLite.Path
		base.PostgresURL = persisted.Database.Postgres.URL
		base.DatabaseURL = persisted.Database.Postgres.URL
		base.MySQLHost = persisted.Database.MySQL.Host
		base.MySQLPort = persisted.Database.MySQL.Port
		base.MySQLName = persisted.Database.MySQL.Name
		base.MySQLUser = persisted.Database.MySQL.User
		base.MySQLPassword = persisted.Database.MySQL.Password
		base.MySQLTLSMode = persisted.Database.MySQL.TLSMode
	}
	base.ObjectBackend = persisted.Object.Backend
	base.ObjectFilesystemRoot = persisted.Object.Filesystem.Path
	base.S3Endpoint = persisted.Object.S3.Endpoint
	base.S3AccessKey = persisted.Object.S3.AccessKey
	base.S3SecretKey = persisted.Object.S3.SecretKey
	base.S3Bucket = persisted.Object.S3.Bucket
	base.S3UseTLS = persisted.Object.S3.UseTLS
	base.S3Region = persisted.Object.S3.Region
	base.S3PathStyle = persisted.Object.S3.PathStyle
	base.S3CreateBucket = persisted.Object.S3.CreateBucket
	return base
}

func writeAtomic(path string, value diskConfig) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".config-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0600); err != nil {
		temporary.Close()
		return err
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return err
	}
	dir, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func readConfigFile(path string) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read configuration file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("read configuration file: configuration must be a regular file")
	}
	if info.Mode().Perm() != 0600 {
		if err := os.Chmod(path, 0600); err != nil {
			return nil, false, fmt.Errorf("secure configuration file permissions: %w", err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("read configuration file: %w", err)
	}
	return data, true, nil
}

func storageEnvironmentConfigured() bool {
	keys := []string{"STORAGE_PROFILE", "DATABASE_BACKEND", "STORE_BACKEND", "DATABASE_URL", "DATABASE_PATH", "DATABASE_DATA_ROOT", "MYSQL_HOST", "MYSQL_PORT", "MYSQL_DATABASE", "MYSQL_USER", "MYSQL_PASSWORD", "MYSQL_TLS_MODE", "OBJECT_BACKEND", "OBJECT_FILESYSTEM_ROOT", "S3_ENDPOINT", "S3_ACCESS_KEY", "S3_SECRET_KEY", "S3_BUCKET", "S3_USE_TLS", "S3_REGION", "S3_PATH_STYLE", "S3_CREATE_BUCKET", "MINIO_ENDPOINT", "MINIO_ACCESS_KEY", "MINIO_SECRET_KEY", "MINIO_BUCKET", "MINIO_USE_SSL"}
	for _, key := range keys {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}

func authConfigured(cfg Config) bool {
	return cfg.AuthUsername != "" && cfg.AuthPassword != ""
}

func developmentLoopback(cfg Config) bool {
	return cfg.AppEnv != "production" && loopbackAddress(cfg.HTTPAddr)
}

func loopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		return false
	}
	ip := net.ParseIP(host)
	if ip != nil {
		return ip.IsLoopback()
	}
	addresses, err := net.LookupIP(host)
	if err != nil || len(addresses) == 0 {
		return false
	}
	for _, address := range addresses {
		if !address.IsLoopback() {
			return false
		}
	}
	return true
}
