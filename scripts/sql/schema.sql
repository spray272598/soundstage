USE soundstage;

CREATE TABLE IF NOT EXISTS rooms (
    id VARCHAR(32) PRIMARY KEY,
    anchor_id VARCHAR(32) NOT NULL,
    title VARCHAR(255) NOT NULL,
    status VARCHAR(16) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at DATETIME NULL,
    ended_at DATETIME NULL,
    INDEX idx_rooms_anchor_id (anchor_id),
    INDEX idx_rooms_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS gifts (
    id VARCHAR(32) PRIMARY KEY,
    name VARCHAR(64) NOT NULL,
    price BIGINT NOT NULL,
    icon VARCHAR(255) NOT NULL,
    status VARCHAR(16) NOT NULL,
    INDEX idx_gifts_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS gift_orders (
    id VARCHAR(32) PRIMARY KEY,
    room_id VARCHAR(32) NOT NULL,
    sender_id VARCHAR(32) NOT NULL,
    gift_id VARCHAR(32) NOT NULL,
    gift_name VARCHAR(64) NOT NULL,
    count INT NOT NULL,
    unit_price BIGINT NOT NULL,
    total_amount BIGINT NOT NULL,
    status VARCHAR(16) NOT NULL,
    idempotency_key VARCHAR(64) NOT NULL,
    created_at DATETIME NOT NULL,
    settled_at DATETIME NULL,
    UNIQUE KEY uk_gift_orders_idempotency (idempotency_key),
    INDEX idx_gift_orders_room (room_id),
    INDEX idx_gift_orders_sender (sender_id),
    INDEX idx_gift_orders_gift (gift_id),
    INDEX idx_gift_orders_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS room_stats (
    room_id VARCHAR(32) PRIMARY KEY,
    likes BIGINT NOT NULL DEFAULT 0,
    gifts_total BIGINT NOT NULL DEFAULT 0,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Seed the gift catalog. Gifts are configured centrally by operations; rooms
-- only decide whether gifting is enabled.
INSERT IGNORE INTO gifts (id, name, price, icon, status) VALUES
    ('g_rose',   'Rose',     10,   '🌹', 'active'),
    ('g_heart',  'Heart',    99,   '💖', 'active'),
    ('g_car',    'Sports Car', 1999, '🏎️', 'active'),
    ('g_rocket', 'Rocket',   9999, '🚀', 'active'),
    ('g_crown',  'Crown',    29999, '👑', 'active'),
    ('g_archive','Archived',  1,    '📦', 'inactive');
