-- ProxPanel SaaS - Database Initialization
-- Creates admin schema and tenant template

-- Extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Admin Schema
CREATE SCHEMA IF NOT EXISTS admin;

-- Super Admins
CREATE TABLE IF NOT EXISTS admin.super_admins (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Tenants
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
);

-- NAS → Tenant mapping for RADIUS routing
CREATE TABLE IF NOT EXISTS admin.nas_tenant_map (
    nas_ip VARCHAR(15) PRIMARY KEY,
    tenant_id INTEGER REFERENCES admin.tenants(id),
    wg_subnet VARCHAR(20)
);

-- Seed default super-admin (password: admin123 — CHANGE IN PRODUCTION)
-- bcrypt hash of 'admin123'
INSERT INTO admin.super_admins (username, password_hash, email)
VALUES ('superadmin', '$2a$10$rZJ3Qj6fj4Q5kqL6Y8Lz2eqXMRxhHvYqFmNhLqj5DzBqGj5pOhxW2', 'admin@proxrad.com')
ON CONFLICT (username) DO NOTHING;

-- Create tenant_template schema (will be populated by the Go app on startup)
CREATE SCHEMA IF NOT EXISTS tenant_template;
