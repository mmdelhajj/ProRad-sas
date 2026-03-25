package database

import (
	"fmt"
	"log"
	"sync"

	"gorm.io/gorm"
)

// tenantDBCache caches per-tenant DB sessions to avoid repeated SET search_path calls
var tenantDBCache sync.Map

// GetTenantDB returns a GORM session scoped to the given tenant schema.
// All queries using this session will operate within that schema.
func GetTenantDB(schemaName string) *gorm.DB {
	return DB.Session(&gorm.Session{NewDB: true}).
		Exec(fmt.Sprintf("SET search_path TO %s, public", schemaName))
}

// SetTenantSchema sets the search_path on an existing DB session
func SetTenantSchema(db *gorm.DB, schemaName string) *gorm.DB {
	return db.Exec(fmt.Sprintf("SET search_path TO %s, public", schemaName))
}

// ProvisionTenantSchema creates a new schema for a tenant by cloning the template
func ProvisionTenantSchema(schemaName string) error {
	log.Printf("SaaS: Provisioning schema %s from template...", schemaName)

	// Create the new schema
	if err := DB.Exec(fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schemaName)).Error; err != nil {
		return fmt.Errorf("failed to create schema %s: %w", schemaName, err)
	}

	// Get list of tables from tenant_template schema
	var tables []string
	if err := DB.Raw(`
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'tenant_template'
		AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`).Pluck("table_name", &tables).Error; err != nil {
		return fmt.Errorf("failed to list template tables: %w", err)
	}

	if len(tables) == 0 {
		return fmt.Errorf("tenant_template schema has no tables - run CreateTenantTemplate first")
	}

	// Clone each table from template to new schema (structure + data)
	for _, table := range tables {
		createSQL := fmt.Sprintf(
			"CREATE TABLE %s.%s (LIKE tenant_template.%s INCLUDING ALL)",
			schemaName, table, table,
		)
		if err := DB.Exec(createSQL).Error; err != nil {
			return fmt.Errorf("failed to create table %s.%s: %w", schemaName, table, err)
		}

		// Copy seed data (e.g. permissions)
		copySQL := fmt.Sprintf(
			"INSERT INTO %s.%s SELECT * FROM tenant_template.%s",
			schemaName, table, table,
		)
		if err := DB.Exec(copySQL).Error; err != nil {
			// Some tables may be empty, that's fine
			log.Printf("SaaS: Note - no seed data for %s.%s (may be empty): %v", schemaName, table, err)
		}
	}

	// Fix sequences - reset all serial columns to start after max id
	var sequences []struct {
		SeqName   string
		TableName string
		ColName   string
	}
	DB.Raw(`
		SELECT s.relname as seq_name, t.relname as table_name, a.attname as col_name
		FROM pg_class s
		JOIN pg_namespace ns ON ns.oid = s.relnamespace
		JOIN pg_depend d ON d.objid = s.oid
		JOIN pg_class t ON t.oid = d.refobjid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = d.refobjsubid
		WHERE ns.nspname = $1 AND s.relkind = 'S'
	`, schemaName).Scan(&sequences)

	for _, seq := range sequences {
		DB.Exec(fmt.Sprintf(
			"SELECT setval('%s.%s', COALESCE((SELECT MAX(%s) FROM %s.%s), 1))",
			schemaName, seq.SeqName, seq.ColName, schemaName, seq.TableName,
		))
	}

	log.Printf("SaaS: Schema %s provisioned with %d tables", schemaName, len(tables))
	return nil
}

// DropTenantSchema drops a tenant's schema (use with caution!)
func DropTenantSchema(schemaName string) error {
	if schemaName == "public" || schemaName == "admin" || schemaName == "tenant_template" {
		return fmt.Errorf("cannot drop protected schema: %s", schemaName)
	}
	return DB.Exec(fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName)).Error
}

// CreateAdminSchema creates the admin schema and its tables
func CreateAdminSchema() error {
	log.Println("SaaS: Creating admin schema...")

	if err := DB.Exec("CREATE SCHEMA IF NOT EXISTS admin").Error; err != nil {
		return fmt.Errorf("failed to create admin schema: %w", err)
	}

	// Create tenants table
	if err := DB.Exec(`
		CREATE TABLE IF NOT EXISTS admin.tenants (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			subdomain VARCHAR(100) UNIQUE NOT NULL,
			custom_domain VARCHAR(255),
			schema_name VARCHAR(100) UNIQUE NOT NULL,
			status VARCHAR(20) DEFAULT 'active',
			plan VARCHAR(50) DEFAULT 'free',
			max_subscribers INTEGER DEFAULT 50,
			max_routers INTEGER DEFAULT 1,

			wg_server_private_key TEXT,
			wg_server_public_key TEXT,
			wg_client_private_key TEXT,
			wg_client_public_key TEXT,
			wg_subnet VARCHAR(20),
			wg_server_ip VARCHAR(15),
			wg_client_ip VARCHAR(15),

			mikrotik_api_ip VARCHAR(15),
			mikrotik_api_port INTEGER DEFAULT 8728,
			mikrotik_api_user VARCHAR(100),
			mikrotik_api_password VARCHAR(255),
			radius_secret VARCHAR(100),

			admin_username VARCHAR(100) NOT NULL,
			admin_password_hash VARCHAR(255) NOT NULL,
			admin_email VARCHAR(255),

			stripe_customer_id VARCHAR(255),
			trial_ends_at TIMESTAMP,
			current_subscriber_count INTEGER DEFAULT 0,

			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)
	`).Error; err != nil {
		return fmt.Errorf("failed to create tenants table: %w", err)
	}

	// Create NAS→tenant mapping table
	if err := DB.Exec(`
		CREATE TABLE IF NOT EXISTS admin.nas_tenant_map (
			nas_ip VARCHAR(15) PRIMARY KEY,
			tenant_id INTEGER REFERENCES admin.tenants(id),
			wg_subnet VARCHAR(20)
		)
	`).Error; err != nil {
		return fmt.Errorf("failed to create nas_tenant_map table: %w", err)
	}

	// Create super_admins table
	if err := DB.Exec(`
		CREATE TABLE IF NOT EXISTS admin.super_admins (
			id SERIAL PRIMARY KEY,
			username VARCHAR(100) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			email VARCHAR(255),
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)
	`).Error; err != nil {
		return fmt.Errorf("failed to create super_admins table: %w", err)
	}

	log.Println("SaaS: Admin schema created successfully")
	return nil
}

// CreateTenantTemplate creates the template schema from schema.sql
func CreateTenantTemplate() error {
	log.Println("SaaS: Creating tenant_template schema...")

	if err := DB.Exec("CREATE SCHEMA IF NOT EXISTS tenant_template").Error; err != nil {
		return fmt.Errorf("failed to create tenant_template schema: %w", err)
	}

	// Set search_path to tenant_template so all CREATE TABLE statements go there
	if err := DB.Exec("SET search_path TO tenant_template, public").Error; err != nil {
		return fmt.Errorf("failed to set search_path: %w", err)
	}

	// The schema.sql will be executed separately - this just ensures the schema exists
	// We'll run the existing AutoMigrate within the tenant_template schema
	log.Println("SaaS: tenant_template schema ready - run schema.sql within it")

	// Reset search_path
	DB.Exec("SET search_path TO public")

	return nil
}

// ListTenantSchemas returns all tenant schema names
func ListTenantSchemas() ([]string, error) {
	var schemas []string
	if err := DB.Raw(`
		SELECT schema_name FROM information_schema.schemata
		WHERE schema_name LIKE 'tenant_%'
		AND schema_name != 'tenant_template'
		ORDER BY schema_name
	`).Pluck("schema_name", &schemas).Error; err != nil {
		return nil, err
	}
	return schemas, nil
}
