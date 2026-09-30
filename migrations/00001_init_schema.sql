-- +goose Up
-- Таблица складов / спа-объектов
CREATE TABLE IF NOT EXISTS locations (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    address TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Таблица поставщиков
CREATE TABLE IF NOT EXISTS suppliers (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    contact_person VARCHAR(255),
    phone VARCHAR(50),
    email VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Таблица товаров (номенклатурный справочник)
CREATE TABLE IF NOT EXISTS products (
    sku VARCHAR(50) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    category VARCHAR(100) NOT NULL,
    unit VARCHAR(20) NOT NULL,
    package_size NUMERIC(14, 4) NOT NULL DEFAULT 1.0000,
    min_order_qty NUMERIC(14, 4) NOT NULL DEFAULT 1.0000,
    lead_time_days INT NOT NULL DEFAULT 7,
    default_supplier_id VARCHAR(50) REFERENCES suppliers(id) ON DELETE SET NULL,
    last_purchase_price NUMERIC(14, 2) NOT NULL DEFAULT 0.00,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Таблица партий поступлений (серии/лоты производителя, независимые от локаций)
-- Для безопасного создания новых партий expiry_date имеет значение по умолчанию +1 год, purchase_price - 0.00
CREATE TABLE IF NOT EXISTS batches (
    id VARCHAR(50) NOT NULL,
    sku VARCHAR(50) NOT NULL REFERENCES products(sku) ON DELETE RESTRICT,
    expiry_date DATE DEFAULT (CURRENT_DATE + INTERVAL '1 year'),
    purchase_price NUMERIC(14, 2) NOT NULL DEFAULT 0.00,
    invoice_no VARCHAR(100),
    received_date DATE NOT NULL DEFAULT CURRENT_DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, sku)
);

-- Таблица обработанных документов (Идемпотентность и защита от повторной загрузки - 409 Conflict)
-- Первичный ключ document_no гарантирует на уровне БД исключение дубликатов даже при параллельных запросах
CREATE TABLE IF NOT EXISTS processed_documents (
    document_no VARCHAR(100) PRIMARY KEY,
    operation_type VARCHAR(20) NOT NULL CHECK (operation_type IN ('receipt', 'consume', 'writeoff', 'return', 'correction')),
    operation_date TIMESTAMPTZ NOT NULL,
    sku VARCHAR(50) NOT NULL REFERENCES products(sku) ON DELETE RESTRICT,
    location_id VARCHAR(50) NOT NULL REFERENCES locations(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Журнал движений товаров (Append-Only Ledger)
-- В рамках одного документа может быть несколько строк списания по FEFO (дробление по партиям)
CREATE TABLE IF NOT EXISTS movements (
    id BIGSERIAL PRIMARY KEY,
    document_no VARCHAR(100) NOT NULL REFERENCES processed_documents(document_no) ON DELETE RESTRICT,
    operation_date TIMESTAMPTZ NOT NULL,
    sku VARCHAR(50) NOT NULL REFERENCES products(sku) ON DELETE RESTRICT,
    location_id VARCHAR(50) NOT NULL REFERENCES locations(id) ON DELETE RESTRICT,
    operation_type VARCHAR(20) NOT NULL CHECK (operation_type IN ('receipt', 'consume', 'writeoff', 'return', 'correction')),
    quantity NUMERIC(14, 4) NOT NULL,
    batch_id VARCHAR(50) NOT NULL,
    note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (batch_id, sku) REFERENCES batches(id, sku) ON DELETE RESTRICT
);

-- Таблица открытых заказов поставщикам (в пути / ожидаемые поставки)
CREATE TABLE IF NOT EXISTS open_orders (
    id VARCHAR(50) PRIMARY KEY,
    sku VARCHAR(50) NOT NULL REFERENCES products(sku) ON DELETE RESTRICT,
    location_id VARCHAR(50) NOT NULL REFERENCES locations(id) ON DELETE RESTRICT,
    supplier_id VARCHAR(50) NOT NULL REFERENCES suppliers(id) ON DELETE RESTRICT,
    quantity NUMERIC(14, 4) NOT NULL,
    order_date DATE NOT NULL,
    expected_delivery_date DATE NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'in_transit' CHECK (status IN ('in_transit', 'delivered', 'cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Индексы для оптимизации выборок и расчетов остатков
CREATE INDEX IF NOT EXISTS idx_movements_document_no ON movements (document_no);
CREATE INDEX IF NOT EXISTS idx_movements_sku_loc_date ON movements (sku, location_id, operation_date);
CREATE INDEX IF NOT EXISTS idx_movements_batch ON movements (batch_id, sku);
CREATE INDEX IF NOT EXISTS idx_movements_type ON movements (operation_type);
CREATE INDEX IF NOT EXISTS idx_batches_expiry ON batches (sku, expiry_date);
CREATE INDEX IF NOT EXISTS idx_open_orders_sku_loc ON open_orders (sku, location_id, status);

-- +goose Down
DROP TABLE IF EXISTS open_orders CASCADE;
DROP TABLE IF EXISTS movements CASCADE;
DROP TABLE IF EXISTS processed_documents CASCADE;
DROP TABLE IF EXISTS batches CASCADE;
DROP TABLE IF EXISTS products CASCADE;
DROP TABLE IF EXISTS suppliers CASCADE;
DROP TABLE IF EXISTS locations CASCADE;
