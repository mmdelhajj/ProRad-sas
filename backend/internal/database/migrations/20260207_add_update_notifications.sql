-- Migration: Add update_notifications table
-- Description: Create table to track update notifications sent to customers
-- Date: 2026-02-07

CREATE TABLE IF NOT EXISTS update_notifications (
    id SERIAL PRIMARY KEY,
    update_id INTEGER NOT NULL,
    license_id INTEGER NOT NULL,
    customer_id INTEGER NOT NULL,
    notification_type VARCHAR(20) NOT NULL CHECK (notification_type IN ('email', 'sms', 'in-app')),
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed', 'read')),
    sent_at TIMESTAMP,
    read_at TIMESTAMP,
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    CONSTRAINT fk_update FOREIGN KEY (update_id) REFERENCES updates(id) ON DELETE CASCADE,
    CONSTRAINT fk_license FOREIGN KEY (license_id) REFERENCES licenses(id) ON DELETE CASCADE,
    CONSTRAINT fk_customer FOREIGN KEY (customer_id) REFERENCES customers(id) ON DELETE CASCADE
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_update_notifications_update_id ON update_notifications(update_id);
CREATE INDEX IF NOT EXISTS idx_update_notifications_license_id ON update_notifications(license_id);
CREATE INDEX IF NOT EXISTS idx_update_notifications_customer_id ON update_notifications(customer_id);
CREATE INDEX IF NOT EXISTS idx_update_notifications_status ON update_notifications(status);
CREATE INDEX IF NOT EXISTS idx_update_notifications_type ON update_notifications(notification_type);
CREATE INDEX IF NOT EXISTS idx_update_notifications_deleted_at ON update_notifications(deleted_at);

-- Add SMTP email settings to system_preferences if not exists
INSERT INTO system_preferences (key, value, description) VALUES
    ('smtp_enabled', 'false', 'Enable SMTP email notifications'),
    ('smtp_host', '', 'SMTP server hostname'),
    ('smtp_port', '587', 'SMTP server port'),
    ('smtp_user', '', 'SMTP username'),
    ('smtp_password', '', 'SMTP password (encrypted)'),
    ('smtp_from_address', 'noreply@proxpanel.com', 'From email address'),
    ('smtp_from_name', 'ProxPanel', 'From name'),
    ('smtp_encryption', 'tls', 'Encryption method: tls, starttls, or none')
ON CONFLICT (key) DO NOTHING;

COMMENT ON TABLE update_notifications IS 'Tracks notifications sent to customers about system updates';
COMMENT ON COLUMN update_notifications.notification_type IS 'Type of notification: email, sms, or in-app';
COMMENT ON COLUMN update_notifications.status IS 'Status: pending (queued), sent (delivered), failed (error), read (customer viewed)';
