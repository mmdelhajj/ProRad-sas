-- ProISP Database Schema
-- This file is used for migrations instead of GORM AutoMigrate to support code obfuscation

-- Extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Permissions
CREATE TABLE IF NOT EXISTS permissions (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    description VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Default Permissions (seed data)
INSERT INTO permissions (name, description) VALUES ('dashboard.view', 'View dashboard') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('dashboard.view_admin', 'View admin dashboard') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('dashboard.view_active_only', 'View only active users in dashboard') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('dashboard.stats', 'View dashboard statistics') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.view', 'View subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.view_all', 'View all subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.create', 'Create subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.edit', 'Edit subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.edit_all', 'Edit all subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.delete', 'Delete subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.delete_all', 'Delete all subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.delete_expired', 'Delete expired subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.delete_all_expired', 'Delete all expired subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.renew', 'Renew subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.renew_all', 'Renew all subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.disconnect', 'Disconnect subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.disconnect_all', 'Disconnect all subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.inactivate', 'Inactivate subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.inactivate_all', 'Inactivate all subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.rename', 'Rename subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.rename_all', 'Rename all subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.change_owner', 'Change subscriber owner') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.change_owner_all', 'Change all subscribers owner') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.change_service', 'Change subscriber service') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.change_service_free', 'Change user service for free') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.change_expiry', 'Change user expiry date') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.change_service_money', 'Change service money for user') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.change_service_money_all', 'Change service money for all users') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.add_days', 'Add days to subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.add_days_all', 'Add days to all subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.add_days_overdue', 'Add overdue days to subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.add_days_overdue_all', 'Add overdue days to all subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.reset_fup', 'Reset FUP quota') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.reset_fup_all', 'Reset FUP quota for all') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.refill_quota', 'Refill monthly quota for user') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.refill_quota_all', 'Refill monthly quota for all users') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.reset_mac', 'Reset MAC address') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.reset_mac_all', 'Reset MAC address for all') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.unbind_mac', 'Unbind user MAC address') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.queue_quota', 'Use queue quota') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.ping', 'Ping subscriber') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.ping_all', 'Ping all subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.view_graph', 'View live user graph') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.view_graph_all', 'View live user graph for all') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.torch', 'View live torch traffic') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.port_check', 'Check subscriber port') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.bandwidth_rules', 'Manage subscriber bandwidth rules') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.view_fup', 'View FUP level in list') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.view_logs', 'View logs for subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.view_logs_all', 'View logs for all subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.bulk_import', 'Bulk import subscribers (CSV)') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.bulk_add', 'Admin add bulk users') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.bulk_update', 'Bulk update subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.bulk_action', 'Bulk actions (renew/disconnect)') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.change_bulk', 'ChangeBulk (admin bulk operations)') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.export', 'Export/download list of subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.export_all', 'Export/download list of all subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.view_archived', 'View archived subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.restore', 'Restore archived subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.allow_refund', 'Allow refund for deleted subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.refund_no_money', 'Stop connection without refund') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.refund_no_money_all', 'Stop all connections without refund') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('subscribers.autorecharge', 'Auto recharge users') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('services.view', 'View/list services') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('services.create', 'Create services') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('services.edit', 'Edit services') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('services.delete', 'Delete services') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('nas.view', 'View/list NAS devices') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('nas.create', 'Create NAS devices') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('nas.edit', 'Edit NAS devices') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('nas.delete', 'Delete NAS devices') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('nas.sync', 'Sync NAS devices') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('nas.test', 'Test NAS connection') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('sessions.view', 'View user sessions') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('sessions.view_all', 'View all users sessions') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('sessions.disconnect', 'Disconnect sessions') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('sessions.view_history', 'View session history') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('resellers.view', 'View resellers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('resellers.view_all', 'View all resellers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('resellers.create', 'Create resellers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('resellers.edit', 'Edit resellers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('resellers.edit_all', 'Edit all resellers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('resellers.delete', 'Delete resellers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('resellers.change_owner', 'Change reseller owner') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('resellers.change_owner_all', 'Change all reseller owner') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('resellers.add_money', 'Add money to resellers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('resellers.add_money_all', 'Add money to all resellers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('resellers.withdraw', 'Withdraw money from resellers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('resellers.withdraw_all', 'Withdraw money from all resellers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('resellers.view_subresellers', 'View sub-resellers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('resellers.view_balance', 'View reseller balance') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('resellers.set_credit', 'Set reseller credit limit') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('resellers.add_support', 'Add/edit/list support users') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('resellers.add_collector', 'Add/edit/list collector users') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('resellers.notification', 'Send notification in user portal') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('resellers.recharge_code', 'Recharge voucher codes for users via API') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('resellers.recharge_code_all', 'Recharge voucher codes for all users via API') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('resellers.billing_add', 'Billing add reseller') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('invoices.view', 'View invoices') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('invoices.create', 'Create invoices') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('invoices.edit', 'Edit invoices') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('invoices.delete', 'Delete invoices') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('invoices.print', 'Print invoices') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('invoices.email', 'Email invoices') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('invoices.mark_paid', 'Mark invoices as paid') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('prepaid.view', 'View/list prepaid cards') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('prepaid.create', 'Generate prepaid cards') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('prepaid.edit', 'Use/edit prepaid cards') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('prepaid.generate', 'Generate prepaid cards for users') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('prepaid.generate_all', 'Generate prepaid cards for all users') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('prepaid.delete', 'Delete prepaid cards') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('prepaid.disable', 'Disable prepaid cards') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('prepaid.print', 'Print prepaid cards') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('prepaid.export', 'Export prepaid cards') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('prepaid.hide_code', 'Hide generate card codes') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('reports.view', 'View reports') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('reports.generate_all', 'Generate all type of reports') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('reports.subscribers', 'View subscriber reports') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('reports.revenue', 'View revenue reports') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('reports.services', 'View service reports') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('reports.usage', 'View usage reports') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('reports.resellers', 'View reseller reports') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('reports.export', 'Export reports') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('transactions.view', 'View transactions') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('transactions.view_all', 'View all transactions') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('transactions.create', 'Create transactions') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('transactions.delete', 'Delete transactions') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('tickets.view', 'View tickets') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('tickets.create', 'Create tickets') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('tickets.edit', 'Edit tickets') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('tickets.delete', 'Delete tickets') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('tickets.reply', 'Reply to tickets') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('tickets.assign', 'Assign tickets') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('tickets.close', 'Close tickets') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('backups.view', 'View backups') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('backups.create', 'Create backups') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('backups.edit', 'Edit backup schedules') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('backups.restore', 'Restore backups') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('backups.delete', 'Delete backups') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('backups.download', 'Download backups') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('settings.view', 'View settings') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('settings.edit', 'Edit settings') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('settings.wan_check', 'Manage WAN management check settings') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('audit.view', 'View audit logs') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('logs.view', 'View system logs') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('users.view', 'View/list users') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('users.create', 'Create users') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('users.edit', 'Edit users') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('users.delete', 'Delete users') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('communication.view', 'View communication rules') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('communication.create', 'Create communication rules') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('communication.edit', 'Edit communication rules') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('communication.delete', 'Delete communication rules') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('bandwidth.view', 'View bandwidth rules') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('bandwidth.create', 'Create bandwidth rules') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('bandwidth.edit', 'Edit bandwidth rules') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('bandwidth.delete', 'Delete bandwidth rules') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('fup.view', 'View FUP counters') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('fup.reset', 'Reset FUP counters') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('sharing.view', 'View sharing detection') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('sharing.scan', 'Run sharing detection scan') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('sharing.settings', 'Edit sharing detection settings') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('cdn.view', 'View CDN settings') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('cdn.create', 'Create CDN entries') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('cdn.edit', 'Edit CDN entries') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('cdn.delete', 'Delete CDN entries') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('permissions.view', 'View permission groups') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('permissions.create', 'Create permission groups') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('permissions.edit', 'Edit permission groups') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('permissions.delete', 'Delete permission groups') ON CONFLICT (name) DO NOTHING;

-- Permission Groups
CREATE TABLE IF NOT EXISTS permission_groups (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    description VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Permission Group Permissions (many-to-many)
CREATE TABLE IF NOT EXISTS permission_group_permissions (
    permission_group_id INTEGER REFERENCES permission_groups(id) ON DELETE CASCADE,
    permission_id INTEGER REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (permission_group_id, permission_id)
);

-- NAS Devices (Network Access Servers)
CREATE TABLE IF NOT EXISTS nas_devices (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    short_name VARCHAR(50),
    ip_address VARCHAR(50) NOT NULL UNIQUE,
    type VARCHAR(50) DEFAULT 'mikrotik',
    description VARCHAR(255),
    secret VARCHAR(100) NOT NULL,
    auth_port INTEGER DEFAULT 1812,
    acct_port INTEGER DEFAULT 1813,
    coa_port INTEGER DEFAULT 1700,
    api_username VARCHAR(100),
    api_password VARCHAR(255),
    api_port INTEGER DEFAULT 8728,
    api_ssl_port INTEGER DEFAULT 8729,
    use_ssl BOOLEAN DEFAULT false,
    ftp_port INTEGER DEFAULT 21,
    subscriber_pools VARCHAR(500),
    allowed_realms VARCHAR(500),
    is_active BOOLEAN DEFAULT true,
    is_online BOOLEAN DEFAULT false,
    last_seen TIMESTAMP,
    version VARCHAR(50),
    active_sessions INTEGER DEFAULT 0,
    total_users INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Switches
CREATE TABLE IF NOT EXISTS switches (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    location VARCHAR(255),
    parent_id INTEGER REFERENCES switches(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Services
CREATE TABLE IF NOT EXISTS services (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    commercial_name VARCHAR(100),
    description TEXT,
    download_speed BIGINT NOT NULL,
    upload_speed BIGINT NOT NULL,
    download_speed_str VARCHAR(50),
    upload_speed_str VARCHAR(50),
    burst_download BIGINT DEFAULT 0,
    burst_upload BIGINT DEFAULT 0,
    burst_threshold BIGINT DEFAULT 0,
    burst_time INTEGER DEFAULT 0,
    daily_quota BIGINT DEFAULT 0,
    monthly_quota BIGINT DEFAULT 0,
    time_quota INTEGER DEFAULT 0,
    fup1_threshold BIGINT DEFAULT 0,
    fup1_download_speed BIGINT DEFAULT 0,
    fup1_upload_speed BIGINT DEFAULT 0,
    fup2_threshold BIGINT DEFAULT 0,
    fup2_download_speed BIGINT DEFAULT 0,
    fup2_upload_speed BIGINT DEFAULT 0,
    fup3_threshold BIGINT DEFAULT 0,
    fup3_download_speed BIGINT DEFAULT 0,
    fup3_upload_speed BIGINT DEFAULT 0,
    fup4_threshold BIGINT DEFAULT 0,
    fup4_download_speed BIGINT DEFAULT 0,
    fup4_upload_speed BIGINT DEFAULT 0,
    fup5_threshold BIGINT DEFAULT 0,
    fup5_download_speed BIGINT DEFAULT 0,
    fup5_upload_speed BIGINT DEFAULT 0,
    fup6_threshold BIGINT DEFAULT 0,
    fup6_download_speed BIGINT DEFAULT 0,
    fup6_upload_speed BIGINT DEFAULT 0,
    monthly_fup1_threshold BIGINT DEFAULT 0,
    monthly_fup1_download_speed BIGINT DEFAULT 0,
    monthly_fup1_upload_speed BIGINT DEFAULT 0,
    monthly_fup2_threshold BIGINT DEFAULT 0,
    monthly_fup2_download_speed BIGINT DEFAULT 0,
    monthly_fup2_upload_speed BIGINT DEFAULT 0,
    monthly_fup3_threshold BIGINT DEFAULT 0,
    monthly_fup3_download_speed BIGINT DEFAULT 0,
    monthly_fup3_upload_speed BIGINT DEFAULT 0,
    monthly_fup4_threshold BIGINT DEFAULT 0,
    monthly_fup4_download_speed BIGINT DEFAULT 0,
    monthly_fup4_upload_speed BIGINT DEFAULT 0,
    monthly_fup5_threshold BIGINT DEFAULT 0,
    monthly_fup5_download_speed BIGINT DEFAULT 0,
    monthly_fup5_upload_speed BIGINT DEFAULT 0,
    monthly_fup6_threshold BIGINT DEFAULT 0,
    monthly_fup6_download_speed BIGINT DEFAULT 0,
    monthly_fup6_upload_speed BIGINT DEFAULT 0,
    price DECIMAL(15,2) NOT NULL,
    day_price DECIMAL(15,2) DEFAULT 0,
    reset_price DECIMAL(15,2) DEFAULT 0,
    expiry_value INTEGER DEFAULT 30,
    expiry_unit INTEGER DEFAULT 1,
    entire_month BOOLEAN DEFAULT false,
    monthly_account BOOLEAN DEFAULT false,
    time_from_hour INTEGER DEFAULT 0,
    time_from_minute INTEGER DEFAULT 0,
    time_to_hour INTEGER DEFAULT 0,
    time_to_minute INTEGER DEFAULT 0,
    time_download_ratio INTEGER DEFAULT 100,
    time_upload_ratio INTEGER DEFAULT 100,
    nas_id INTEGER,
    pool_name VARCHAR(100),
    address_list_in VARCHAR(100),
    address_list_out VARCHAR(100),
    queue_type VARCHAR(50) DEFAULT 'simple',
    time_based_speed_enabled BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Users
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    password_plain VARCHAR(255),
    email VARCHAR(255),
    phone VARCHAR(50),
    full_name VARCHAR(255),
    user_type INTEGER DEFAULT 1,
    is_active BOOLEAN DEFAULT true,
    force_password_change BOOLEAN DEFAULT false,
    last_login TIMESTAMP,
    two_factor_enabled BOOLEAN DEFAULT false,
    two_factor_secret VARCHAR(255),
    reseller_id INTEGER,
    permission_group INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Resellers
CREATE TABLE IF NOT EXISTS resellers (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    balance DECIMAL(15,2) DEFAULT 0,
    credit DECIMAL(15,2) DEFAULT 0,
    address VARCHAR(500),
    parent_id INTEGER REFERENCES resellers(id),
    permission_group INTEGER,
    bandwidth_rule_id INTEGER,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Update users foreign key
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_reseller_id_fkey;
ALTER TABLE users ADD CONSTRAINT users_reseller_id_fkey FOREIGN KEY (reseller_id) REFERENCES resellers(id) ON DELETE SET NULL;

-- Reseller Services
CREATE TABLE IF NOT EXISTS reseller_services (
    id SERIAL PRIMARY KEY,
    reseller_id INTEGER NOT NULL,
    service_id INTEGER NOT NULL,
    price DECIMAL(15,2) NOT NULL,
    day_price DECIMAL(15,2) DEFAULT 0,
    is_enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(reseller_id, service_id)
);

-- Reseller NAS
CREATE TABLE IF NOT EXISTS reseller_nas (
    id SERIAL PRIMARY KEY,
    reseller_id INTEGER NOT NULL,
    nas_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(reseller_id, nas_id)
);

-- Reseller Service Limits (per-service subscriber caps)
CREATE TABLE IF NOT EXISTS reseller_service_limits (
    id SERIAL PRIMARY KEY,
    reseller_id INTEGER NOT NULL,
    service_id INTEGER NOT NULL,
    max_subscribers INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(reseller_id, service_id)
);

-- Subscribers
CREATE TABLE IF NOT EXISTS subscribers (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) NOT NULL,
    password VARCHAR(255) NOT NULL,
    password_plain VARCHAR(255),
    full_name VARCHAR(255),
    email VARCHAR(255),
    phone VARCHAR(50),
    address VARCHAR(500),
    region VARCHAR(100),
    building VARCHAR(100),
    nationality VARCHAR(100),
    country VARCHAR(100),
    note TEXT,
    service_id INTEGER NOT NULL,
    status INTEGER DEFAULT 1,
    expiry_date TIMESTAMP,
    due_date TIMESTAMP,
    price DECIMAL(15,2),
    override_price BOOLEAN DEFAULT false,
    auto_renew BOOLEAN DEFAULT false,
    daily_download_used BIGINT DEFAULT 0,
    daily_upload_used BIGINT DEFAULT 0,
    monthly_download_used BIGINT DEFAULT 0,
    monthly_upload_used BIGINT DEFAULT 0,
    fup_level INTEGER DEFAULT 0,
    monthly_fup_level INTEGER DEFAULT 0,
    last_daily_reset TIMESTAMP,
    last_monthly_reset TIMESTAMP,
    last_session_download BIGINT DEFAULT 0,
    last_session_upload BIGINT DEFAULT 0,
    last_quota_sync TIMESTAMP,
    last_bypass_cdn_bytes BIGINT DEFAULT 0,
    daily_quota_used BIGINT DEFAULT 0,
    monthly_quota_used BIGINT DEFAULT 0,
    last_quota_reset TIMESTAMP,
    mac_address VARCHAR(50),
    ip_address VARCHAR(50),
    static_ip VARCHAR(50),
    save_mac BOOLEAN DEFAULT true,
    nas_id INTEGER,
    switch_id INTEGER,
    latitude DECIMAL(10,8),
    longitude DECIMAL(11,8),
    reseller_id INTEGER NOT NULL,
    collector_id INTEGER,
    is_online BOOLEAN DEFAULT false,
    last_seen TIMESTAMP,
    session_id VARCHAR(100),
    simultaneous_sessions INTEGER DEFAULT 1,
    auto_recharge BOOLEAN DEFAULT false,
    auto_recharge_days INTEGER DEFAULT 0,
    whatsapp_notifications BOOLEAN DEFAULT false,
    wan_check_status VARCHAR(20) DEFAULT 'unchecked',
    port_open BOOLEAN DEFAULT false,
    deleted_by_id INTEGER,
    deleted_by_name VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_subscribers_mac ON subscribers(mac_address);
CREATE INDEX IF NOT EXISTS idx_subscribers_online ON subscribers(is_online);
CREATE INDEX IF NOT EXISTS idx_subscribers_reseller ON subscribers(reseller_id);
CREATE INDEX IF NOT EXISTS idx_subscribers_service ON subscribers(service_id);
CREATE INDEX IF NOT EXISTS idx_subscribers_nas ON subscribers(nas_id);
CREATE INDEX IF NOT EXISTS idx_subscribers_status ON subscribers(status);
CREATE INDEX IF NOT EXISTS idx_subscribers_username ON subscribers(username);
CREATE INDEX IF NOT EXISTS idx_subscribers_expiry ON subscribers(expiry_date);
CREATE INDEX IF NOT EXISTS idx_subscribers_deleted ON subscribers(deleted_at);

-- Subscriber Bandwidth Rules
CREATE TABLE IF NOT EXISTS subscriber_bandwidth_rules (
    id SERIAL PRIMARY KEY,
    subscriber_id INTEGER NOT NULL,
    rule_type VARCHAR(20) NOT NULL,
    enabled BOOLEAN DEFAULT true,
    download_speed INTEGER DEFAULT 0,
    upload_speed INTEGER DEFAULT 0,
    cdn_id INTEGER DEFAULT 0,
    cdn_name VARCHAR(100),
    duration VARCHAR(20),
    expires_at TIMESTAMP,
    priority INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- RADIUS Tables
CREATE TABLE IF NOT EXISTS radcheck (
    id SERIAL PRIMARY KEY,
    username VARCHAR(64) NOT NULL DEFAULT '',
    attribute VARCHAR(64) NOT NULL DEFAULT '',
    op CHAR(2) NOT NULL DEFAULT ':=',
    value VARCHAR(253) NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_radcheck_username ON radcheck(username);

CREATE TABLE IF NOT EXISTS radreply (
    id SERIAL PRIMARY KEY,
    username VARCHAR(64) NOT NULL DEFAULT '',
    attribute VARCHAR(64) NOT NULL DEFAULT '',
    op CHAR(2) NOT NULL DEFAULT ':=',
    value VARCHAR(253) NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_radreply_username ON radreply(username);
CREATE INDEX IF NOT EXISTS idx_radreply_user_attr ON radreply(username, attribute);

CREATE TABLE IF NOT EXISTS radgroupcheck (
    id SERIAL PRIMARY KEY,
    groupname VARCHAR(64) NOT NULL DEFAULT '',
    attribute VARCHAR(64) NOT NULL DEFAULT '',
    op CHAR(2) NOT NULL DEFAULT ':=',
    value VARCHAR(253) NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS radgroupreply (
    id SERIAL PRIMARY KEY,
    groupname VARCHAR(64) NOT NULL DEFAULT '',
    attribute VARCHAR(64) NOT NULL DEFAULT '',
    op CHAR(2) NOT NULL DEFAULT ':=',
    value VARCHAR(253) NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS radusergroup (
    id SERIAL PRIMARY KEY,
    username VARCHAR(64) NOT NULL DEFAULT '',
    groupname VARCHAR(64) NOT NULL DEFAULT '',
    priority INTEGER DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_radusergroup_username ON radusergroup(username);

CREATE TABLE IF NOT EXISTS radacct (
    radacctid BIGSERIAL PRIMARY KEY,
    acctsessionid VARCHAR(64) NOT NULL DEFAULT '',
    acctuniqueid VARCHAR(32) NOT NULL UNIQUE DEFAULT '',
    username VARCHAR(64) NOT NULL DEFAULT '',
    realm VARCHAR(64) DEFAULT '',
    nasipaddress VARCHAR(15) NOT NULL DEFAULT '',
    nasportid VARCHAR(50),
    nasporttype VARCHAR(32),
    acctstarttime TIMESTAMP,
    acctupdatetime TIMESTAMP,
    acctstoptime TIMESTAMP,
    acctinterval INTEGER,
    acctsessiontime INTEGER,
    acctauthentic VARCHAR(32),
    connectinfo_start VARCHAR(50),
    connectinfo_stop VARCHAR(50),
    acctinputoctets BIGINT,
    acctoutputoctets BIGINT,
    calledstationid VARCHAR(50) DEFAULT '',
    callingstationid VARCHAR(50) DEFAULT '',
    acctterminatecause VARCHAR(32) DEFAULT '',
    servicetype VARCHAR(32),
    framedprotocol VARCHAR(32),
    framedipaddress VARCHAR(15) DEFAULT '',
    framedipv6address VARCHAR(45) DEFAULT '',
    framedipv6prefix VARCHAR(45) DEFAULT '',
    framedinterfaceid VARCHAR(44) DEFAULT '',
    delegatedipv6prefix VARCHAR(45) DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_radacct_username ON radacct(username);
CREATE INDEX IF NOT EXISTS idx_radacct_start ON radacct(acctstarttime);
CREATE INDEX IF NOT EXISTS idx_radacct_stop ON radacct(acctstoptime);
CREATE INDEX IF NOT EXISTS idx_radacct_session ON radacct(acctsessionid, username);
CREATE INDEX IF NOT EXISTS idx_radacct_nasip ON radacct(nasipaddress);
CREATE INDEX IF NOT EXISTS idx_radacct_framedip ON radacct(framedipaddress);

CREATE TABLE IF NOT EXISTS radpostauth (
    id SERIAL PRIMARY KEY,
    username VARCHAR(64) NOT NULL DEFAULT '',
    pass VARCHAR(64) NOT NULL DEFAULT '',
    reply VARCHAR(32) NOT NULL DEFAULT '',
    callingstationid VARCHAR(50) DEFAULT '',
    authdate TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    class VARCHAR(64) DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_radpostauth_username ON radpostauth(username);
CREATE INDEX IF NOT EXISTS idx_radpostauth_authdate ON radpostauth(authdate);

-- Performance indexes for frequently queried tables
CREATE INDEX IF NOT EXISTS idx_sub_bw_rules_subscriber_id ON subscriber_bandwidth_rules(subscriber_id);
CREATE INDEX IF NOT EXISTS idx_sub_bw_rules_is_active ON subscriber_bandwidth_rules(is_active);
CREATE INDEX IF NOT EXISTS idx_service_cdns_service_id ON service_cdns(service_id);
CREATE INDEX IF NOT EXISTS idx_communication_rules_trigger ON communication_rules(trigger_event);
CREATE INDEX IF NOT EXISTS idx_communication_rules_active ON communication_rules(is_active);
CREATE INDEX IF NOT EXISTS idx_bandwidth_rules_service_id ON bandwidth_rules(service_id);
CREATE INDEX IF NOT EXISTS idx_bandwidth_rules_is_active ON bandwidth_rules(is_active);

-- Transactions
CREATE TABLE IF NOT EXISTS transactions (
    id SERIAL PRIMARY KEY,
    type VARCHAR(50) NOT NULL,
    amount DECIMAL(15,2) NOT NULL,
    balance_before DECIMAL(15,2),
    balance_after DECIMAL(15,2),
    description VARCHAR(500),
    old_service_name VARCHAR(100),
    new_service_name VARCHAR(100),
    service_name VARCHAR(100),
    reseller_id INTEGER NOT NULL,
    subscriber_id INTEGER,
    target_reseller_id INTEGER,
    permission_group INTEGER,
    ip_address VARCHAR(50),
    user_agent VARCHAR(255),
    created_by INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_transactions_type ON transactions(type);
CREATE INDEX IF NOT EXISTS idx_transactions_reseller ON transactions(reseller_id);
CREATE INDEX IF NOT EXISTS idx_transactions_created ON transactions(created_at);
CREATE INDEX IF NOT EXISTS idx_transactions_subscriber ON transactions(subscriber_id);

-- Invoices
CREATE TABLE IF NOT EXISTS invoices (
    id SERIAL PRIMARY KEY,
    invoice_number VARCHAR(50) NOT NULL UNIQUE,
    subscriber_id INTEGER NOT NULL,
    reseller_id INTEGER NOT NULL,
    sub_total DECIMAL(15,2),
    discount DECIMAL(15,2) DEFAULT 0,
    tax DECIMAL(15,2) DEFAULT 0,
    total DECIMAL(15,2) NOT NULL,
    amount_paid DECIMAL(15,2) DEFAULT 0,
    status VARCHAR(20) DEFAULT 'pending',
    due_date TIMESTAMP,
    paid_date TIMESTAMP,
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_invoices_subscriber ON invoices(subscriber_id);
CREATE INDEX IF NOT EXISTS idx_invoices_status ON invoices(status);
CREATE INDEX IF NOT EXISTS idx_invoices_reseller ON invoices(reseller_id);

-- Invoice Items
CREATE TABLE IF NOT EXISTS invoice_items (
    id SERIAL PRIMARY KEY,
    invoice_id INTEGER NOT NULL,
    description VARCHAR(255) NOT NULL,
    quantity INTEGER DEFAULT 1,
    unit_price DECIMAL(15,2) NOT NULL,
    total DECIMAL(15,2) NOT NULL
);

-- Payments
CREATE TABLE IF NOT EXISTS payments (
    id SERIAL PRIMARY KEY,
    invoice_id INTEGER,
    subscriber_id INTEGER NOT NULL,
    reseller_id INTEGER NOT NULL,
    collector_id INTEGER,
    amount DECIMAL(15,2) NOT NULL,
    method VARCHAR(50) DEFAULT 'cash',
    reference VARCHAR(100),
    notes TEXT,
    status VARCHAR(20) DEFAULT 'completed',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Prepaid Cards
CREATE TABLE IF NOT EXISTS prepaid_cards (
    id SERIAL PRIMARY KEY,
    code VARCHAR(50) NOT NULL UNIQUE,
    pin VARCHAR(20),
    service_id INTEGER NOT NULL,
    reseller_id INTEGER NOT NULL,
    value DECIMAL(15,2) NOT NULL,
    days INTEGER DEFAULT 30,
    quota_refill BIGINT DEFAULT 0,
    is_used BOOLEAN DEFAULT false,
    used_by INTEGER,
    used_at TIMESTAMP,
    is_active BOOLEAN DEFAULT true,
    expiry_date TIMESTAMP,
    batch_id VARCHAR(50),
    batch_number INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by INTEGER
);
CREATE INDEX IF NOT EXISTS idx_prepaid_used ON prepaid_cards(is_used);
CREATE INDEX IF NOT EXISTS idx_prepaid_batch ON prepaid_cards(batch_id);

-- Static IP Prices
CREATE TABLE IF NOT EXISTS static_ip_prices (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    price DECIMAL(15,2) NOT NULL,
    days INTEGER DEFAULT 30,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Static IP Rentals
CREATE TABLE IF NOT EXISTS static_ip_rentals (
    id SERIAL PRIMARY KEY,
    subscriber_id INTEGER NOT NULL,
    ip_address VARCHAR(50) NOT NULL UNIQUE,
    price_id INTEGER NOT NULL,
    start_date TIMESTAMP,
    expiry_date TIMESTAMP,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Audit Logs
CREATE TABLE IF NOT EXISTS audit_logs (
    id SERIAL PRIMARY KEY,
    user_id INTEGER,
    username VARCHAR(100),
    user_type INTEGER,
    action VARCHAR(50) NOT NULL,
    entity_type VARCHAR(50),
    entity_id INTEGER,
    entity_name VARCHAR(100),
    old_value JSONB,
    new_value JSONB,
    description VARCHAR(500),
    ip_address VARCHAR(50),
    user_agent VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at);

-- Notifications
CREATE TABLE IF NOT EXISTS notifications (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    type VARCHAR(50) DEFAULT 'info',
    is_active BOOLEAN DEFAULT true,
    start_date TIMESTAMP,
    end_date TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Tickets
CREATE TABLE IF NOT EXISTS tickets (
    id SERIAL PRIMARY KEY,
    ticket_number VARCHAR(20) NOT NULL UNIQUE,
    subject VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    description TEXT NOT NULL,
    status VARCHAR(20) DEFAULT 'open',
    priority VARCHAR(20) DEFAULT 'normal',
    category VARCHAR(50),
    creator_type VARCHAR(20),
    subscriber_id INTEGER,
    reseller_id INTEGER,
    permission_group INTEGER,
    created_by INTEGER,
    assigned_to INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    closed_at TIMESTAMP
);

-- Ticket Replies
CREATE TABLE IF NOT EXISTS ticket_replies (
    id SERIAL PRIMARY KEY,
    ticket_id INTEGER NOT NULL,
    message TEXT NOT NULL,
    user_id INTEGER,
    is_internal BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Communication Templates
CREATE TABLE IF NOT EXISTS communication_templates (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(20) NOT NULL,
    subject VARCHAR(255),
    body TEXT NOT NULL,
    variables TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Communication Rules
CREATE TABLE IF NOT EXISTS communication_rules (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    trigger_event VARCHAR(50) NOT NULL,
    channel VARCHAR(20) NOT NULL DEFAULT 'sms',
    days_before INTEGER DEFAULT 0,
    template TEXT,
    enabled BOOLEAN DEFAULT true,
    send_to_reseller BOOLEAN DEFAULT false,
    reseller_id INTEGER,
    permission_group INTEGER,
    fup_levels VARCHAR(20) DEFAULT '1,2,3,4,5,6',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
ALTER TABLE communication_rules ADD COLUMN IF NOT EXISTS fup_levels VARCHAR(20) DEFAULT '1,2,3,4,5,6';

-- Communication Logs
CREATE TABLE IF NOT EXISTS communication_logs (
    id SERIAL PRIMARY KEY,
    type VARCHAR(20) NOT NULL,
    recipient VARCHAR(255) NOT NULL,
    subject VARCHAR(255),
    message TEXT,
    status VARCHAR(20) DEFAULT 'pending',
    error_message VARCHAR(500),
    subscriber_id INTEGER,
    reseller_id INTEGER,
    permission_group INTEGER,
    rule_id INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    sent_at TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_comm_type ON communication_logs(type);
CREATE INDEX IF NOT EXISTS idx_comm_created ON communication_logs(created_at);

-- Daily Quotas
CREATE TABLE IF NOT EXISTS daily_quotas (
    id SERIAL PRIMARY KEY,
    subscriber_id INTEGER NOT NULL,
    date VARCHAR(10) NOT NULL,
    download BIGINT DEFAULT 0,
    upload BIGINT DEFAULT 0,
    total BIGINT DEFAULT 0,
    UNIQUE(subscriber_id, date)
);

-- Monthly Quotas
CREATE TABLE IF NOT EXISTS monthly_quotas (
    id SERIAL PRIMARY KEY,
    subscriber_id INTEGER NOT NULL,
    month VARCHAR(7) NOT NULL,
    download BIGINT DEFAULT 0,
    upload BIGINT DEFAULT 0,
    total BIGINT DEFAULT 0,
    UNIQUE(subscriber_id, month)
);

-- System Preferences
CREATE TABLE IF NOT EXISTS system_preferences (
    id SERIAL PRIMARY KEY,
    key VARCHAR(100) NOT NULL UNIQUE,
    value TEXT,
    value_type VARCHAR(20) DEFAULT 'string'
);

-- Sessions
CREATE TABLE IF NOT EXISTS sessions (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) NOT NULL,
    subscriber_id INTEGER,
    nas_id INTEGER,
    nas_ip_address VARCHAR(50),
    framed_ip_address VARCHAR(50),
    calling_station_id VARCHAR(50),
    acct_session_id VARCHAR(100) UNIQUE,
    session_time BIGINT,
    input_octets BIGINT,
    output_octets BIGINT,
    status VARCHAR(20) DEFAULT 'online',
    start_time TIMESTAMP,
    update_time TIMESTAMP,
    stop_time TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_sessions_username ON sessions(username);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);

-- CDNs
CREATE TABLE IF NOT EXISTS cdns (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    description VARCHAR(255),
    subnets TEXT,
    color VARCHAR(20) DEFAULT '#EF4444',
    nas_ids VARCHAR(500),
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Service CDNs
CREATE TABLE IF NOT EXISTS service_cdns (
    id SERIAL PRIMARY KEY,
    service_id INTEGER NOT NULL,
    cdn_id INTEGER NOT NULL,
    speed_limit BIGINT DEFAULT 0,
    bypass_quota BOOLEAN DEFAULT false,
    pcq_enabled BOOLEAN DEFAULT false,
    pcq_limit INTEGER DEFAULT 50,
    pcq_total_limit INTEGER DEFAULT 2000,
    pcq_nas_id INTEGER,
    pcq_target_pools VARCHAR(500),
    is_active BOOLEAN DEFAULT true,
    time_from_hour INTEGER DEFAULT 0,
    time_from_minute INTEGER DEFAULT 0,
    time_to_hour INTEGER DEFAULT 0,
    time_to_minute INTEGER DEFAULT 0,
    time_speed_ratio INTEGER DEFAULT 100,
    time_based_speed_enabled BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(service_id, cdn_id)
);

-- CDN Port Rules
CREATE TABLE IF NOT EXISTS cdn_port_rules (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    port VARCHAR(50),
    direction VARCHAR(10) DEFAULT 'both',
    dscp_value INTEGER,
    speed_mbps BIGINT NOT NULL DEFAULT 5,
    nas_id INTEGER,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);
ALTER TABLE cdn_port_rules ADD COLUMN IF NOT EXISTS dscp_value INTEGER;
ALTER TABLE cdn_port_rules ALTER COLUMN port DROP NOT NULL;
ALTER TABLE cdn_port_rules ADD COLUMN IF NOT EXISTS show_in_graph BOOLEAN DEFAULT false;
ALTER TABLE cdn_port_rules ADD COLUMN IF NOT EXISTS color VARCHAR(20) DEFAULT '#8B5CF6';

-- Bandwidth Rules
CREATE TABLE IF NOT EXISTS bandwidth_rules (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    trigger_type VARCHAR(20) NOT NULL DEFAULT 'time',
    start_time VARCHAR(10),
    end_time VARCHAR(10),
    days_of_week JSON,
    upload_multiplier INTEGER DEFAULT 100,
    download_multiplier INTEGER DEFAULT 100,
    service_ids JSON,
    priority INTEGER DEFAULT 10,
    enabled BOOLEAN DEFAULT true,
    auto_apply BOOLEAN DEFAULT false
);

-- CDN Bandwidth Rules
CREATE TABLE IF NOT EXISTS cdn_bandwidth_rules (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    start_time VARCHAR(10),
    end_time VARCHAR(10),
    days_of_week JSON,
    speed_multiplier INTEGER DEFAULT 100,
    cdn_ids JSON,
    service_ids JSON,
    priority INTEGER DEFAULT 10,
    enabled BOOLEAN DEFAULT true,
    auto_apply BOOLEAN DEFAULT false
);

-- Backup Schedules
CREATE TABLE IF NOT EXISTS backup_schedules (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    is_enabled BOOLEAN DEFAULT true,
    backup_type VARCHAR(20) DEFAULT 'full',
    frequency VARCHAR(20) NOT NULL,
    day_of_week INTEGER DEFAULT 0,
    day_of_month INTEGER DEFAULT 1,
    time_of_day VARCHAR(5) DEFAULT '02:00',
    retention INTEGER DEFAULT 7,
    storage_type VARCHAR(20) DEFAULT 'local',
    local_path VARCHAR(255),
    ftp_enabled BOOLEAN DEFAULT false,
    ftp_host VARCHAR(255),
    ftp_port INTEGER DEFAULT 21,
    ftp_username VARCHAR(100),
    ftp_password VARCHAR(255),
    ftp_path VARCHAR(255) DEFAULT '/backups',
    ftp_passive BOOLEAN DEFAULT true,
    ftp_tls BOOLEAN DEFAULT false,
    cloud_enabled BOOLEAN DEFAULT false,
    include_mikrotik BOOLEAN DEFAULT false,
    last_run_at TIMESTAMP,
    last_status VARCHAR(20),
    last_error VARCHAR(500),
    last_backup_file VARCHAR(255),
    next_run_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Backup Logs
CREATE TABLE IF NOT EXISTS backup_logs (
    id SERIAL PRIMARY KEY,
    schedule_id INTEGER,
    schedule_name VARCHAR(100),
    backup_type VARCHAR(20),
    filename VARCHAR(255),
    file_size BIGINT,
    storage_type VARCHAR(20),
    storage_path VARCHAR(500),
    status VARCHAR(20),
    error_message VARCHAR(500),
    duration INTEGER,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    created_by_id INTEGER,
    created_by_name VARCHAR(100)
);

-- Sharing Detection History
CREATE TABLE IF NOT EXISTS sharing_detections (
    id SERIAL PRIMARY KEY,
    subscriber_id INTEGER NOT NULL,
    username VARCHAR(100),
    full_name VARCHAR(200),
    ip_address VARCHAR(45),
    service_name VARCHAR(100),
    nas_id INTEGER,
    nas_name VARCHAR(100),
    connection_count INTEGER DEFAULT 0,
    unique_destinations INTEGER DEFAULT 0,
    ttl_values TEXT,
    ttl_status VARCHAR(50),
    suspicion_level VARCHAR(20),
    confidence_score INTEGER DEFAULT 0,
    reasons TEXT,
    detected_at TIMESTAMP,
    scan_type VARCHAR(20) DEFAULT 'automatic',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sharing_detections_subscriber_id ON sharing_detections(subscriber_id);
CREATE INDEX IF NOT EXISTS idx_sharing_detections_username ON sharing_detections(username);
CREATE INDEX IF NOT EXISTS idx_sharing_detections_suspicion_level ON sharing_detections(suspicion_level);
CREATE INDEX IF NOT EXISTS idx_sharing_detections_detected_at ON sharing_detections(detected_at);

-- Sharing Detection Settings
CREATE TABLE IF NOT EXISTS sharing_detection_settings (
    id SERIAL PRIMARY KEY,
    enabled BOOLEAN DEFAULT true,
    scan_time VARCHAR(5) DEFAULT '03:00',
    retention_days INTEGER DEFAULT 30,
    min_suspicion_level VARCHAR(20) DEFAULT 'medium',
    connection_threshold INTEGER DEFAULT 500,
    notify_on_high_risk BOOLEAN DEFAULT false,
    auto_suspend_repeat BOOLEAN DEFAULT false,
    repeat_threshold INTEGER DEFAULT 5,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert default sharing detection settings
INSERT INTO sharing_detection_settings (enabled, scan_time, retention_days, min_suspicion_level, connection_threshold)
SELECT true, '03:00', 30, 'medium', 500
WHERE NOT EXISTS (SELECT 1 FROM sharing_detection_settings LIMIT 1);

-- Sharing Detection: automated action settings columns
ALTER TABLE sharing_detection_settings ADD COLUMN IF NOT EXISTS auto_flag_enabled BOOLEAN DEFAULT false;
ALTER TABLE sharing_detection_settings ADD COLUMN IF NOT EXISTS auto_flag_threshold INTEGER DEFAULT 70;
ALTER TABLE sharing_detection_settings ADD COLUMN IF NOT EXISTS speed_reduction_enabled BOOLEAN DEFAULT false;
ALTER TABLE sharing_detection_settings ADD COLUMN IF NOT EXISTS speed_reduction_threshold INTEGER DEFAULT 80;
ALTER TABLE sharing_detection_settings ADD COLUMN IF NOT EXISTS speed_reduction_percent INTEGER DEFAULT 50;
ALTER TABLE sharing_detection_settings ADD COLUMN IF NOT EXISTS whatsapp_notify_enabled BOOLEAN DEFAULT false;
ALTER TABLE sharing_detection_settings ADD COLUMN IF NOT EXISTS whatsapp_notify_threshold INTEGER DEFAULT 60;
ALTER TABLE sharing_detection_settings ADD COLUMN IF NOT EXISTS whatsapp_notify_template TEXT DEFAULT '';
ALTER TABLE sharing_detection_settings ADD COLUMN IF NOT EXISTS score_enabled BOOLEAN DEFAULT true;

-- Sharing Detection: new factors on detections
ALTER TABLE sharing_detections ADD COLUMN IF NOT EXISTS traffic_anomaly_factor INTEGER DEFAULT 0;
ALTER TABLE sharing_detections ADD COLUMN IF NOT EXISTS mac_diversity_factor INTEGER DEFAULT 0;
ALTER TABLE sharing_detections ADD COLUMN IF NOT EXISTS service_speed INTEGER DEFAULT 0;

-- Sharing Detection: subscriber whitelist
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS sharing_whitelisted BOOLEAN DEFAULT false;
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS sharing_whitelist_reason VARCHAR(50) DEFAULT '';

-- Subscriber Sharing Scores (monthly evaluation)
CREATE TABLE IF NOT EXISTS subscriber_sharing_scores (
    id SERIAL PRIMARY KEY,
    subscriber_id INTEGER NOT NULL,
    month VARCHAR(7) NOT NULL,
    score INTEGER DEFAULT 0,
    category VARCHAR(20) DEFAULT 'good',
    detection_count INTEGER DEFAULT 0,
    avg_confidence REAL DEFAULT 0,
    peak_confidence INTEGER DEFAULT 0,
    trend VARCHAR(20) DEFAULT 'stable',
    previous_score INTEGER DEFAULT 0,
    traffic_anomaly_score INTEGER DEFAULT 0,
    mac_diversity_score INTEGER DEFAULT 0,
    fup_pattern_score INTEGER DEFAULT 0,
    computed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(subscriber_id, month)
);

CREATE INDEX IF NOT EXISTS idx_sharing_scores_subscriber ON subscriber_sharing_scores(subscriber_id);
CREATE INDEX IF NOT EXISTS idx_sharing_scores_month ON subscriber_sharing_scores(month);
CREATE INDEX IF NOT EXISTS idx_sharing_scores_category ON subscriber_sharing_scores(category);

-- Sharing Action Logs
CREATE TABLE IF NOT EXISTS sharing_action_logs (
    id SERIAL PRIMARY KEY,
    subscriber_id INTEGER NOT NULL,
    username VARCHAR(100),
    action_type VARCHAR(30) NOT NULL,
    trigger_score INTEGER DEFAULT 0,
    trigger_category VARCHAR(20),
    details TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sharing_action_logs_subscriber ON sharing_action_logs(subscriber_id);
CREATE INDEX IF NOT EXISTS idx_sharing_action_logs_created ON sharing_action_logs(created_at);

-- Add force_password_change column if not exists
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'users' AND column_name = 'force_password_change') THEN
        ALTER TABLE users ADD COLUMN force_password_change BOOLEAN DEFAULT false;
    END IF;
END $$;

-- Add is_enabled column to backup_schedules if not exists (renamed from is_active)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'backup_schedules' AND column_name = 'is_enabled') THEN
        IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'backup_schedules' AND column_name = 'is_active') THEN
            ALTER TABLE backup_schedules RENAME COLUMN is_active TO is_enabled;
        ELSE
            ALTER TABLE backup_schedules ADD COLUMN is_enabled BOOLEAN DEFAULT true;
        END IF;
    END IF;
END $$;

-- Add cloud_enabled column to backup_schedules if not exists
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'backup_schedules' AND column_name = 'cloud_enabled') THEN
        ALTER TABLE backup_schedules ADD COLUMN cloud_enabled BOOLEAN DEFAULT false;
    END IF;
END $$;

-- Add include_mikrotik column to backup_schedules if not exists
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'backup_schedules' AND column_name = 'include_mikrotik') THEN
        ALTER TABLE backup_schedules ADD COLUMN include_mikrotik BOOLEAN DEFAULT false;
    END IF;
END $$;

-- HA Cluster Configuration
CREATE TABLE IF NOT EXISTS cluster_config (
    id SERIAL PRIMARY KEY,
    cluster_id VARCHAR(50) UNIQUE,
    cluster_secret VARCHAR(100),
    server_role VARCHAR(20) DEFAULT 'standalone',
    server_ip VARCHAR(45),
    server_name VARCHAR(100),
    main_server_ip VARCHAR(45),
    main_server_port INTEGER DEFAULT 8080,
    db_replication_enabled BOOLEAN DEFAULT false,
    db_replication_status VARCHAR(20) DEFAULT 'offline',
    db_replication_lag INTEGER DEFAULT 0,
    db_sync_port INTEGER DEFAULT 5433,
    redis_replication_enabled BOOLEAN DEFAULT false,
    redis_replication_status VARCHAR(20) DEFAULT 'offline',
    radius_enabled BOOLEAN DEFAULT true,
    radius_role VARCHAR(20) DEFAULT 'primary',
    api_enabled BOOLEAN DEFAULT true,
    api_role VARCHAR(20) DEFAULT 'active',
    auto_failover_enabled BOOLEAN DEFAULT true,
    failover_priority INTEGER DEFAULT 1,
    is_active BOOLEAN DEFAULT false,
    last_heartbeat TIMESTAMP,
    last_sync_time TIMESTAMP,
    database_id VARCHAR(100),
    hardware_id VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- HA Cluster Nodes (tracked by main server)
CREATE TABLE IF NOT EXISTS cluster_nodes (
    id SERIAL PRIMARY KEY,
    cluster_id VARCHAR(50),
    hardware_id VARCHAR(100) UNIQUE,
    server_role VARCHAR(20),
    server_name VARCHAR(100),
    server_ip VARCHAR(45),
    version VARCHAR(20),
    status VARCHAR(20) DEFAULT 'offline',
    db_sync_status VARCHAR(20) DEFAULT 'offline',
    redis_sync_status VARCHAR(20) DEFAULT 'offline',
    db_replication_lag INTEGER DEFAULT 0,
    cpu_usage DECIMAL(5,2) DEFAULT 0,
    memory_usage DECIMAL(5,2) DEFAULT 0,
    cpu_cores INTEGER DEFAULT 0,
    ram_gb INTEGER DEFAULT 0,
    disk_usage DECIMAL(5,2) DEFAULT 0,
    last_heartbeat TIMESTAMP,
    last_sync_time TIMESTAMP,
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    database_id VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_cluster_nodes_cluster_id ON cluster_nodes(cluster_id);
CREATE INDEX IF NOT EXISTS idx_cluster_nodes_status ON cluster_nodes(status);

-- HA Cluster Events (audit log)
CREATE TABLE IF NOT EXISTS cluster_events (
    id SERIAL PRIMARY KEY,
    cluster_id VARCHAR(50),
    event_type VARCHAR(50),
    node_id INTEGER,
    node_ip VARCHAR(45),
    node_role VARCHAR(20),
    description VARCHAR(500),
    severity VARCHAR(20),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_cluster_events_cluster_id ON cluster_events(cluster_id);
CREATE INDEX IF NOT EXISTS idx_cluster_events_event_type ON cluster_events(event_type);
CREATE INDEX IF NOT EXISTS idx_cluster_events_severity ON cluster_events(severity);

-- IP Pool Assignments (ProISP-managed IP allocation)
CREATE TABLE IF NOT EXISTS ip_pool_assignments (
    id SERIAL PRIMARY KEY,
    ip_address VARCHAR(15) NOT NULL,
    pool_name VARCHAR(64) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'available',
    username VARCHAR(100),
    subscriber_id INTEGER,
    nas_id INTEGER,
    session_id VARCHAR(100),
    assigned_at TIMESTAMP,
    released_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ip_pool_ip_address ON ip_pool_assignments(ip_address);
CREATE INDEX IF NOT EXISTS idx_ip_pool_pool_name ON ip_pool_assignments(pool_name);
CREATE INDEX IF NOT EXISTS idx_ip_pool_status ON ip_pool_assignments(status);
CREATE INDEX IF NOT EXISTS idx_ip_pool_username ON ip_pool_assignments(username);

-- Create default admin user if not exists
INSERT INTO users (username, password, password_plain, full_name, user_type, is_active, force_password_change)
SELECT 'admin', '$2b$12$gTtR9Kks5AVDaIJSeMAiZ.eFvZF9CYHzdTJEhxzzWMpZzW5Vvgqoq', 'admin123', 'Administrator', 4, true, true
WHERE NOT EXISTS (SELECT 1 FROM users WHERE username = 'admin');

-- Create default reseller for admin if not exists
INSERT INTO resellers (user_id, name, balance, is_active)
SELECT u.id, 'Main Admin', 999999999, true
FROM users u
WHERE u.username = 'admin'
AND NOT EXISTS (SELECT 1 FROM resellers WHERE user_id = u.id);

-- Update admin user with reseller_id
UPDATE users SET reseller_id = (SELECT id FROM resellers WHERE name = 'Main Admin' LIMIT 1)
WHERE username = 'admin' AND reseller_id IS NULL;

-- Add nas_id column to services if not exists (for remembering which NAS was used to select pool)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'services' AND column_name = 'nas_id') THEN
        ALTER TABLE services ADD COLUMN nas_id INTEGER;
    END IF;
END $$;

-- Reseller WhatsApp (per-reseller WhatsApp notifications)
INSERT INTO permissions (name, description) VALUES ('notifications.whatsapp', 'Send WhatsApp notifications to own subscribers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('communication.access_module', 'Access Communication Rules module') ON CONFLICT (name) DO NOTHING;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'resellers' AND column_name = 'whatsapp_account_unique') THEN
        ALTER TABLE resellers ADD COLUMN whatsapp_account_unique VARCHAR(100);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'resellers' AND column_name = 'whatsapp_phone') THEN
        ALTER TABLE resellers ADD COLUMN whatsapp_phone VARCHAR(50);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'resellers' AND column_name = 'whatsapp_enabled') THEN
        ALTER TABLE resellers ADD COLUMN whatsapp_enabled BOOLEAN DEFAULT false;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'resellers' AND column_name = 'whatsapp_trial_start') THEN
        ALTER TABLE resellers ADD COLUMN whatsapp_trial_start TIMESTAMPTZ;
    END IF;
END $$;

ALTER TABLE resellers ADD COLUMN IF NOT EXISTS rebrand_enabled BOOLEAN DEFAULT false;
ALTER TABLE resellers ADD COLUMN IF NOT EXISTS custom_domain VARCHAR(255);

-- Reseller branding customization
CREATE TABLE IF NOT EXISTS reseller_brandings (
    id SERIAL PRIMARY KEY,
    reseller_id INTEGER NOT NULL UNIQUE REFERENCES resellers(id) ON DELETE CASCADE,
    company_name VARCHAR(255),
    logo_path VARCHAR(500),
    primary_color VARCHAR(20) DEFAULT '#2563eb',
    footer_text VARCHAR(500),
    tagline VARCHAR(500),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Collection assignments (collector feature)
CREATE TABLE IF NOT EXISTS collection_assignments (
    id               SERIAL PRIMARY KEY,
    collector_id     INTEGER NOT NULL REFERENCES users(id),
    subscriber_id    INTEGER NOT NULL REFERENCES subscribers(id),
    reseller_id      INTEGER NOT NULL DEFAULT 0,
    invoice_id       INTEGER REFERENCES invoices(id),
    status           VARCHAR(20) NOT NULL DEFAULT 'pending',
    auto_renew       BOOLEAN NOT NULL DEFAULT false,
    send_notification BOOLEAN NOT NULL DEFAULT true,
    amount           DECIMAL(15,2) DEFAULT 0,
    notes            TEXT,
    collected_at     TIMESTAMP WITH TIME ZONE,
    payment_id       INTEGER REFERENCES payments(id),
    created_by       INTEGER NOT NULL,
    created_at       TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Collector permissions
INSERT INTO permissions (name, description) VALUES ('collector.view', 'Collector sees own assignments') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('collector.collect', 'Collector can mark collected') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('collectors.view', 'View collector list and assignments') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('collectors.create', 'Create collection assignments') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('collectors.reports', 'View collector performance reports') ON CONFLICT (name) DO NOTHING;

-- Daily usage history (saves daily counters before reset for accurate past-day charts)
CREATE TABLE IF NOT EXISTS daily_usage_history (
    id BIGSERIAL PRIMARY KEY,
    subscriber_id INTEGER NOT NULL,
    date DATE NOT NULL,
    download_bytes BIGINT DEFAULT 0,
    upload_bytes BIGINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(subscriber_id, date)
);

-- WAN Management Check columns (v1.0.352+)
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS wan_check_status VARCHAR(20) DEFAULT 'unchecked';
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS port_open BOOLEAN DEFAULT false;

-- Deletion tracking columns (v1.0.352+)
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS deleted_by_id INTEGER;
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS deleted_by_name VARCHAR(100);

-- Auto-invoice toggle (v1.0.361+)
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS auto_invoice BOOLEAN DEFAULT false;

-- Invoice billing period for dedup + auto-generated flag (v1.0.361+)
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS billing_period_start TIMESTAMP;
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS billing_period_end TIMESTAMP;
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS auto_generated BOOLEAN DEFAULT false;

-- Default invoice generation setting
INSERT INTO system_preferences (key, value, value_type)
VALUES ('invoice_days_before_expiry', '7', 'int') ON CONFLICT (key) DO NOTHING;

-- Add callingstationid to radpostauth for existing installs (v1.0.362+)
ALTER TABLE radpostauth ADD COLUMN IF NOT EXISTS callingstationid VARCHAR(50) DEFAULT '';

-- Notification Banners (v1.0.363+)
CREATE TABLE IF NOT EXISTS notification_banners (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    banner_type VARCHAR(20) DEFAULT 'info',
    target VARCHAR(20) DEFAULT 'all',
    target_ids TEXT DEFAULT '',
    start_date TIMESTAMP NOT NULL,
    end_date TIMESTAMP NOT NULL,
    dismissible BOOLEAN DEFAULT true,
    enabled BOOLEAN DEFAULT true,
    created_by_id INTEGER,
    created_by_name VARCHAR(100),
    reseller_id INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

INSERT INTO permissions (name, description) VALUES ('communication.notifications', 'Manage notification banners') ON CONFLICT (name) DO NOTHING;

-- FUP 4-6 columns for services table (upgrade from 3 to 6 FUP tiers)
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'services' AND column_name = 'fup4_threshold') THEN
        ALTER TABLE services ADD COLUMN fup4_threshold BIGINT DEFAULT 0;
        ALTER TABLE services ADD COLUMN fup4_download_speed BIGINT DEFAULT 0;
        ALTER TABLE services ADD COLUMN fup4_upload_speed BIGINT DEFAULT 0;
        ALTER TABLE services ADD COLUMN fup5_threshold BIGINT DEFAULT 0;
        ALTER TABLE services ADD COLUMN fup5_download_speed BIGINT DEFAULT 0;
        ALTER TABLE services ADD COLUMN fup5_upload_speed BIGINT DEFAULT 0;
        ALTER TABLE services ADD COLUMN fup6_threshold BIGINT DEFAULT 0;
        ALTER TABLE services ADD COLUMN fup6_download_speed BIGINT DEFAULT 0;
        ALTER TABLE services ADD COLUMN fup6_upload_speed BIGINT DEFAULT 0;
        ALTER TABLE services ADD COLUMN monthly_fup4_threshold BIGINT DEFAULT 0;
        ALTER TABLE services ADD COLUMN monthly_fup4_download_speed BIGINT DEFAULT 0;
        ALTER TABLE services ADD COLUMN monthly_fup4_upload_speed BIGINT DEFAULT 0;
        ALTER TABLE services ADD COLUMN monthly_fup5_threshold BIGINT DEFAULT 0;
        ALTER TABLE services ADD COLUMN monthly_fup5_download_speed BIGINT DEFAULT 0;
        ALTER TABLE services ADD COLUMN monthly_fup5_upload_speed BIGINT DEFAULT 0;
        ALTER TABLE services ADD COLUMN monthly_fup6_threshold BIGINT DEFAULT 0;
        ALTER TABLE services ADD COLUMN monthly_fup6_download_speed BIGINT DEFAULT 0;
        ALTER TABLE services ADD COLUMN monthly_fup6_upload_speed BIGINT DEFAULT 0;
    END IF;
END $$;

-- CDN FUP columns for services
ALTER TABLE services ADD COLUMN IF NOT EXISTS cdn_fup_enabled BOOLEAN DEFAULT false;
ALTER TABLE services ADD COLUMN IF NOT EXISTS cdn_fup1_threshold BIGINT DEFAULT 0;
ALTER TABLE services ADD COLUMN IF NOT EXISTS cdn_fup1_download_speed BIGINT DEFAULT 0;
ALTER TABLE services ADD COLUMN IF NOT EXISTS cdn_fup1_upload_speed BIGINT DEFAULT 0;
ALTER TABLE services ADD COLUMN IF NOT EXISTS cdn_fup2_threshold BIGINT DEFAULT 0;
ALTER TABLE services ADD COLUMN IF NOT EXISTS cdn_fup2_download_speed BIGINT DEFAULT 0;
ALTER TABLE services ADD COLUMN IF NOT EXISTS cdn_fup2_upload_speed BIGINT DEFAULT 0;
ALTER TABLE services ADD COLUMN IF NOT EXISTS cdn_fup3_threshold BIGINT DEFAULT 0;
ALTER TABLE services ADD COLUMN IF NOT EXISTS cdn_fup3_download_speed BIGINT DEFAULT 0;
ALTER TABLE services ADD COLUMN IF NOT EXISTS cdn_fup3_upload_speed BIGINT DEFAULT 0;
ALTER TABLE services ADD COLUMN IF NOT EXISTS cdn_monthly_fup1_threshold BIGINT DEFAULT 0;
ALTER TABLE services ADD COLUMN IF NOT EXISTS cdn_monthly_fup1_download_speed BIGINT DEFAULT 0;
ALTER TABLE services ADD COLUMN IF NOT EXISTS cdn_monthly_fup1_upload_speed BIGINT DEFAULT 0;
ALTER TABLE services ADD COLUMN IF NOT EXISTS cdn_monthly_fup2_threshold BIGINT DEFAULT 0;
ALTER TABLE services ADD COLUMN IF NOT EXISTS cdn_monthly_fup2_download_speed BIGINT DEFAULT 0;
ALTER TABLE services ADD COLUMN IF NOT EXISTS cdn_monthly_fup2_upload_speed BIGINT DEFAULT 0;
ALTER TABLE services ADD COLUMN IF NOT EXISTS cdn_monthly_fup3_threshold BIGINT DEFAULT 0;
ALTER TABLE services ADD COLUMN IF NOT EXISTS cdn_monthly_fup3_download_speed BIGINT DEFAULT 0;
ALTER TABLE services ADD COLUMN IF NOT EXISTS cdn_monthly_fup3_upload_speed BIGINT DEFAULT 0;

-- CDN FUP tracking columns for subscribers
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS cdn_daily_download_used BIGINT DEFAULT 0;
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS cdn_daily_upload_used BIGINT DEFAULT 0;
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS cdn_monthly_download_used BIGINT DEFAULT 0;
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS cdn_monthly_upload_used BIGINT DEFAULT 0;
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS cdn_fup_level INTEGER DEFAULT 0;
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS cdn_monthly_fup_level INTEGER DEFAULT 0;

-- CDN Torch interface for NAS
ALTER TABLE nas_devices ADD COLUMN IF NOT EXISTS cdn_torch_interface VARCHAR(100);

-- ============================================
-- Feature: Reseller Commissions (Feature 8)
-- ============================================
CREATE TABLE IF NOT EXISTS reseller_commissions (
    id SERIAL PRIMARY KEY,
    reseller_id INTEGER NOT NULL,
    subscriber_id INTEGER,
    transaction_id INTEGER,
    amount REAL DEFAULT 0,
    commission_percent REAL DEFAULT 0,
    commission_amount REAL DEFAULT 0,
    type VARCHAR(20) DEFAULT 'payment',
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

ALTER TABLE resellers ADD COLUMN IF NOT EXISTS commission_percent REAL DEFAULT 0;
ALTER TABLE resellers ADD COLUMN IF NOT EXISTS commission_enabled BOOLEAN DEFAULT false;

-- ============================================
-- Feature: Maintenance Windows (Feature 11)
-- ============================================
CREATE TABLE IF NOT EXISTS maintenance_windows (
    id SERIAL PRIMARY KEY,
    title VARCHAR(200) NOT NULL,
    message TEXT,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    is_active BOOLEAN DEFAULT true,
    notify_subscribers BOOLEAN DEFAULT false,
    created_by INTEGER,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- ============================================
-- Feature: Churn Prediction (Feature 17)
-- ============================================
CREATE TABLE IF NOT EXISTS churn_scores (
    id SERIAL PRIMARY KEY,
    subscriber_id INTEGER NOT NULL,
    score INTEGER DEFAULT 0,
    risk_level VARCHAR(20) DEFAULT 'low',
    factors TEXT,
    days_until_expiry INTEGER DEFAULT 0,
    usage_trend VARCHAR(20) DEFAULT 'stable',
    ticket_count INTEGER DEFAULT 0,
    payment_delays INTEGER DEFAULT 0,
    computed_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(subscriber_id)
);

-- ============================================
-- Feature: Overdue Suspend + Auto-Archive (Features 1, 2)
-- ============================================
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS predicted_fup_date DATE;
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS usage_velocity REAL DEFAULT 0;
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS churn_risk VARCHAR(20) DEFAULT 'low';
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS churn_score INTEGER DEFAULT 0;

-- ============================================
-- Feature: Prorated Billing (Feature 16)
-- ============================================
INSERT INTO system_preferences (key, value) VALUES ('prorate_enabled', 'false') ON CONFLICT (key) DO NOTHING;
INSERT INTO system_preferences (key, value) VALUES ('prorate_method', 'daily') ON CONFLICT (key) DO NOTHING;

-- ============================================
-- Feature: Overdue/Archive Settings (Features 1, 2)
-- ============================================
INSERT INTO system_preferences (key, value) VALUES ('overdue_suspend_enabled', 'false') ON CONFLICT (key) DO NOTHING;
INSERT INTO system_preferences (key, value) VALUES ('overdue_suspend_days', '7') ON CONFLICT (key) DO NOTHING;
INSERT INTO system_preferences (key, value) VALUES ('auto_archive_enabled', 'false') ON CONFLICT (key) DO NOTHING;
INSERT INTO system_preferences (key, value) VALUES ('auto_archive_days', '30') ON CONFLICT (key) DO NOTHING;

-- ============================================
-- Feature: Maintenance Mode (Feature 11)
-- ============================================
INSERT INTO system_preferences (key, value) VALUES ('maintenance_mode', 'false') ON CONFLICT (key) DO NOTHING;
INSERT INTO system_preferences (key, value) VALUES ('maintenance_message', '') ON CONFLICT (key) DO NOTHING;

-- ============================================
-- Feature: RADIUS Logs (persistent event log)
-- ============================================
CREATE TABLE IF NOT EXISTS radius_logs (
    id BIGSERIAL PRIMARY KEY,
    event_type VARCHAR(20) NOT NULL DEFAULT '',
    username VARCHAR(100) DEFAULT '',
    nas_ip VARCHAR(50) DEFAULT '',
    client_ip VARCHAR(50) DEFAULT '',
    mac_address VARCHAR(50) DEFAULT '',
    reason VARCHAR(200) DEFAULT '',
    duration_ms INTEGER DEFAULT 0,
    session_id VARCHAR(64) DEFAULT '',
    details TEXT DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_radius_logs_created ON radius_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_radius_logs_username ON radius_logs(username);
CREATE INDEX IF NOT EXISTS idx_radius_logs_event ON radius_logs(event_type);

-- ============================================
-- Feature: System Logs (API + service events)
-- ============================================
CREATE TABLE IF NOT EXISTS system_logs (
    id BIGSERIAL PRIMARY KEY,
    level VARCHAR(10) NOT NULL DEFAULT 'info',
    module VARCHAR(50) NOT NULL DEFAULT '',
    message TEXT NOT NULL DEFAULT '',
    details TEXT DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_system_logs_created ON system_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_system_logs_level ON system_logs(level);
CREATE INDEX IF NOT EXISTS idx_system_logs_module ON system_logs(module);

-- ============================================
-- Feature: API Keys (External API access)
-- ============================================
CREATE TABLE IF NOT EXISTS api_keys (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id),
    name VARCHAR(100) NOT NULL DEFAULT '',
    key_prefix VARCHAR(8) NOT NULL DEFAULT '',
    key_hash VARCHAR(128) NOT NULL DEFAULT '',
    scopes TEXT NOT NULL DEFAULT 'read',
    is_active BOOLEAN DEFAULT true,
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_user ON api_keys(user_id);

CREATE TABLE IF NOT EXISTS api_key_logs (
    id BIGSERIAL PRIMARY KEY,
    api_key_id INTEGER NOT NULL,
    method VARCHAR(10) NOT NULL DEFAULT '',
    path VARCHAR(255) NOT NULL DEFAULT '',
    status_code INTEGER DEFAULT 0,
    ip_address VARCHAR(50) DEFAULT '',
    duration_ms INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_api_key_logs_key ON api_key_logs(api_key_id);
CREATE INDEX IF NOT EXISTS idx_api_key_logs_created ON api_key_logs(created_at DESC);

-- Public IP Pools
CREATE TABLE IF NOT EXISTS public_ip_pools (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    cidr VARCHAR(50) NOT NULL,
    ip_version INTEGER NOT NULL DEFAULT 4,
    gateway VARCHAR(50),
    monthly_price DECIMAL(10,2) DEFAULT 0,
    description TEXT,
    is_active BOOLEAN DEFAULT true,
    total_ips INTEGER DEFAULT 0,
    used_ips INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Public IP Assignments
CREATE TABLE IF NOT EXISTS public_ip_assignments (
    id SERIAL PRIMARY KEY,
    pool_id INTEGER REFERENCES public_ip_pools(id),
    subscriber_id INTEGER REFERENCES subscribers(id),
    ip_address VARCHAR(50) NOT NULL,
    ip_version INTEGER NOT NULL DEFAULT 4,
    status VARCHAR(20) DEFAULT 'active',
    assigned_at TIMESTAMPTZ DEFAULT NOW(),
    released_at TIMESTAMPTZ,
    last_billed_at TIMESTAMPTZ,
    next_billing_at TIMESTAMPTZ,
    monthly_price DECIMAL(10,2) DEFAULT 0,
    notes TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_public_ip_assignments_pool ON public_ip_assignments(pool_id);
CREATE INDEX IF NOT EXISTS idx_public_ip_assignments_subscriber ON public_ip_assignments(subscriber_id);
CREATE INDEX IF NOT EXISTS idx_public_ip_assignments_status ON public_ip_assignments(status);
CREATE INDEX IF NOT EXISTS idx_public_ip_assignments_billing ON public_ip_assignments(next_billing_at) WHERE status = 'active';
CREATE UNIQUE INDEX IF NOT EXISTS idx_public_ip_assignments_ip_unique ON public_ip_assignments(ip_address) WHERE status IN ('active', 'reserved');

-- Public IP Permissions
INSERT INTO permissions (name, description) VALUES ('public_ips.view', 'View public IP pools and assignments') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('public_ips.manage', 'Manage public IP pools and assignments') ON CONFLICT (name) DO NOTHING;

-- ============================================
-- Feature: Subscriber Wallet / Balance System
-- ============================================
ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS balance DECIMAL(15,2) DEFAULT 0;

-- ============================================
-- Feature: Bandwidth Manager
-- ============================================
CREATE TABLE IF NOT EXISTS bandwidth_customers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    contact_person VARCHAR(200),
    phone VARCHAR(50),
    email VARCHAR(200),
    address TEXT,
    notes TEXT,

    -- Connection
    ip_address VARCHAR(45) NOT NULL,
    subnet_mask VARCHAR(5) DEFAULT '/32',
    gateway VARCHAR(45),
    nas_id INTEGER REFERENCES nas_devices(id),
    interface VARCHAR(50),
    vlan_id INTEGER DEFAULT 0,
    queue_name VARCHAR(100),

    -- Public IP
    public_ip VARCHAR(500),
    public_subnet VARCHAR(5),
    public_gateway VARCHAR(45),

    -- Speed (kb)
    download_speed INTEGER NOT NULL DEFAULT 0,
    upload_speed INTEGER NOT NULL DEFAULT 0,
    cdn_download_speed INTEGER DEFAULT 0,
    cdn_upload_speed INTEGER DEFAULT 0,
    speed_source VARCHAR(20) DEFAULT 'queue',

    -- Burst
    burst_enabled BOOLEAN DEFAULT false,
    burst_download INTEGER DEFAULT 0,
    burst_upload INTEGER DEFAULT 0,
    burst_threshold_dl INTEGER DEFAULT 0,
    burst_threshold_ul INTEGER DEFAULT 0,
    burst_time INTEGER DEFAULT 10,

    -- FUP
    fup_enabled BOOLEAN DEFAULT false,
    daily_quota BIGINT DEFAULT 0,
    fup1_threshold BIGINT DEFAULT 0,
    fup1_speed INTEGER DEFAULT 0,
    fup2_threshold BIGINT DEFAULT 0,
    fup2_speed INTEGER DEFAULT 0,
    fup3_threshold BIGINT DEFAULT 0,
    fup3_speed INTEGER DEFAULT 0,
    monthly_quota BIGINT DEFAULT 0,
    monthly_fup1_threshold BIGINT DEFAULT 0,
    monthly_fup1_speed INTEGER DEFAULT 0,
    monthly_fup2_threshold BIGINT DEFAULT 0,
    monthly_fup2_speed INTEGER DEFAULT 0,
    monthly_fup3_threshold BIGINT DEFAULT 0,
    monthly_fup3_speed INTEGER DEFAULT 0,
    fup_level INTEGER DEFAULT 0,
    monthly_fup_level INTEGER DEFAULT 0,

    -- Usage tracking
    daily_download_used BIGINT DEFAULT 0,
    daily_upload_used BIGINT DEFAULT 0,
    monthly_download_used BIGINT DEFAULT 0,
    monthly_upload_used BIGINT DEFAULT 0,
    last_queue_bytes_in BIGINT DEFAULT 0,
    last_queue_bytes_out BIGINT DEFAULT 0,
    total_download BIGINT DEFAULT 0,
    total_upload BIGINT DEFAULT 0,

    -- Billing
    price DECIMAL(10,2) DEFAULT 0,
    billing_cycle VARCHAR(20) DEFAULT 'monthly',
    start_date DATE,
    expiry_date DATE,
    auto_renew BOOLEAN DEFAULT true,

    -- Status
    status VARCHAR(20) DEFAULT 'active',
    is_online BOOLEAN DEFAULT false,
    last_seen TIMESTAMPTZ,

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    last_daily_reset TIMESTAMPTZ,
    last_monthly_reset TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_bw_customers_ip ON bandwidth_customers(ip_address);
CREATE INDEX IF NOT EXISTS idx_bw_customers_nas ON bandwidth_customers(nas_id);
CREATE INDEX IF NOT EXISTS idx_bw_customers_status ON bandwidth_customers(status);
CREATE INDEX IF NOT EXISTS idx_bw_customers_online ON bandwidth_customers(is_online);
CREATE INDEX IF NOT EXISTS idx_bw_customers_deleted ON bandwidth_customers(deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_bw_customers_ip_unique ON bandwidth_customers(ip_address) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS bw_daily_usage_history (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER NOT NULL REFERENCES bandwidth_customers(id),
    date DATE NOT NULL,
    download_bytes BIGINT DEFAULT 0,
    upload_bytes BIGINT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_bw_usage_history_customer ON bw_daily_usage_history(customer_id);
CREATE INDEX IF NOT EXISTS idx_bw_usage_history_date ON bw_daily_usage_history(date);

-- IP Block Management (IPAM)
CREATE TABLE IF NOT EXISTS bw_ip_blocks (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    cidr VARCHAR(50) NOT NULL,
    gateway VARCHAR(45),
    subnet_mask VARCHAR(20),
    description TEXT,
    total_ips INTEGER DEFAULT 0,
    used_ips INTEGER DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_bw_ip_blocks_deleted ON bw_ip_blocks(deleted_at);

CREATE TABLE IF NOT EXISTS bw_ip_allocations (
    id SERIAL PRIMARY KEY,
    block_id INTEGER NOT NULL REFERENCES bw_ip_blocks(id),
    customer_id INTEGER REFERENCES bandwidth_customers(id),
    ip_address VARCHAR(45) NOT NULL,
    status VARCHAR(20) DEFAULT 'available',
    notes TEXT,
    assigned_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_bw_ip_alloc_ip ON bw_ip_allocations(ip_address);
CREATE INDEX IF NOT EXISTS idx_bw_ip_alloc_block ON bw_ip_allocations(block_id);
CREATE INDEX IF NOT EXISTS idx_bw_ip_alloc_customer ON bw_ip_allocations(customer_id);
CREATE INDEX IF NOT EXISTS idx_bw_ip_alloc_status ON bw_ip_allocations(status);

-- Add IP block reference columns to bandwidth_customers
ALTER TABLE bandwidth_customers ADD COLUMN IF NOT EXISTS ip_block_id INTEGER REFERENCES bw_ip_blocks(id);
ALTER TABLE bandwidth_customers ADD COLUMN IF NOT EXISTS ip_allocation_id INTEGER REFERENCES bw_ip_allocations(id);

-- Hourly bandwidth history
CREATE TABLE IF NOT EXISTS bw_hourly_usage (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER NOT NULL REFERENCES bandwidth_customers(id),
    hour TIMESTAMPTZ NOT NULL,
    download_bytes BIGINT DEFAULT 0,
    upload_bytes BIGINT DEFAULT 0,
    peak_download_kbps BIGINT DEFAULT 0,
    peak_upload_kbps BIGINT DEFAULT 0,
    sample_count INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_bw_hourly_customer_hour ON bw_hourly_usage(customer_id, hour);
CREATE INDEX IF NOT EXISTS idx_bw_hourly_hour ON bw_hourly_usage(hour);

-- Session tracking
CREATE TABLE IF NOT EXISTS bw_sessions (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER NOT NULL REFERENCES bandwidth_customers(id),
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    duration_sec INTEGER DEFAULT 0,
    download_bytes BIGINT DEFAULT 0,
    upload_bytes BIGINT DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_bw_sessions_customer ON bw_sessions(customer_id);
CREATE INDEX IF NOT EXISTS idx_bw_sessions_started ON bw_sessions(started_at);
CREATE INDEX IF NOT EXISTS idx_bw_sessions_ended ON bw_sessions(ended_at);

-- Bandwidth Manager Permissions
INSERT INTO permissions (name, description) VALUES ('bandwidth_customers.view', 'View bandwidth customers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('bandwidth_customers.create', 'Create bandwidth customers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('bandwidth_customers.edit', 'Edit bandwidth customers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('bandwidth_customers.delete', 'Delete bandwidth customers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('bandwidth_customers.suspend', 'Suspend/unsuspend bandwidth customers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('bandwidth_customers.reset_fup', 'Reset FUP for bandwidth customers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('bandwidth_customers.change_speed', 'Change speed for bandwidth customers') ON CONFLICT (name) DO NOTHING;
INSERT INTO permissions (name, description) VALUES ('bandwidth_customers.view_graph', 'View live bandwidth graph') ON CONFLICT (name) DO NOTHING;
