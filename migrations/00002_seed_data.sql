-- +goose Up
-- 1. Склады / СПА-объекты сети Mountain & Sea Spa
INSERT INTO locations (id, name, address) VALUES
('MS-01', 'Mountain & Sea Spa - Центральный', 'г. Сочи, пос. Красная Поляна, ул. Олимпийская, 12'),
('MS-02', 'Mountain & Sea Spa - Приморский', 'г. Сочи, Курортный проспект, 89')
ON CONFLICT (id) DO NOTHING;

-- 2. Справочник поставщиков
INSERT INTO suppliers (id, name, contact_person, phone, email) VALUES
('SUP-01', 'Арома-Ойл Рус', 'Иванов Сергей', '+7 (495) 123-45-67', 'order@aromaoil.ru'),
('SUP-02', 'Премиум Косметикс', 'Смирнова Елена', '+7 (862) 987-65-43', 'b2b@premium-cosm.ru'),
('SUP-03', 'СПА Трейд Холдинг', 'Волков Дмитрий', '+7 (812) 555-44-33', 'supply@spatrade.ru')
ON CONFLICT (id) DO NOTHING;

-- 3. Каталог товаров
INSERT INTO products (sku, name, category, unit, package_size, min_order_qty, lead_time_days, default_supplier_id, last_purchase_price, is_active) VALUES
('OIL-001', 'Массажное масло базовое (миндаль)', 'Масла и эмульсии', 'л', 5.0000, 25.0000, 7, 'SUP-01', 1259.05, true),
('CRM-002', 'Крем питательный для тела (манго)', 'Кремы и лосьоны', 'кг', 2.0000, 10.0000, 10, 'SUP-02', 2450.00, true),
('SCR-003', 'Скраб солевой детокс (эвкалипт)', 'Скрабы и пилинги', 'кг', 1.0000, 5.0000, 5, 'SUP-03', 890.00, true),
('SHP-004', 'Простыни одноразовые спа 70x200', 'Расходные материалы', 'рул', 10.0000, 20.0000, 3, 'SUP-03', 450.00, true),
('OIL-005', 'Эфирное масло лемонграсса', 'Ароматерапия', 'фл', 1.0000, 5.0000, 14, 'SUP-01', 650.00, true)
ON CONFLICT (sku) DO NOTHING;

-- 4. Партии поступлений (привязаны к SKU, без location_id и initial_quantity)
INSERT INTO batches (id, sku, expiry_date, purchase_price, invoice_no, received_date) VALUES
('BATCH-2026-05', 'OIL-001', '2026-11-20', 1250.00, 'INV-2026-0510', '2026-05-10'),
('BATCH-2026-07', 'OIL-001', '2027-07-15', 1259.05, 'INV-2026-0720', '2026-07-20'),
('BATCH-2025-OLD', 'OIL-001', '2026-08-01', 1180.00, 'INV-2025-0801', '2025-08-01'),

-- CRM-002:
('BATCH-CRM-01', 'CRM-002', '2026-12-10', 2450.00, 'INV-CRM-01', '2026-06-15'),

-- SCR-003:
('BATCH-SCR-EXP', 'SCR-003', '2026-10-15', 890.00, 'INV-SCR-EXP', '2026-04-10'),
('BATCH-SCR-02', 'SCR-003', '2027-05-20', 890.00, 'INV-SCR-02', '2026-08-05'),

-- SHP-004:
('BATCH-SHP-01', 'SHP-004', '2029-01-01', 450.00, 'INV-SHP-01', '2026-06-01'),

-- OIL-005:
('BATCH-LEM-01', 'OIL-005', '2027-10-01', 650.00, 'INV-LEM-01', '2026-05-01')
ON CONFLICT (id, sku) DO NOTHING;

-- 5. Учет обработанных документов (Защита от повторной обработки - 409 Conflict)
INSERT INTO processed_documents (document_no, operation_type, operation_date, sku, location_id) VALUES
('DOC-REC-OIL-05', 'receipt', '2026-05-10 10:00:00+03', 'OIL-001', 'MS-01'),
('DOC-REC-OIL-07', 'receipt', '2026-07-20 11:30:00+03', 'OIL-001', 'MS-01'),
('DOC-REC-OIL-OLD', 'receipt', '2025-08-01 09:00:00+03', 'OIL-001', 'MS-01'),
('DOC-WOF-OIL-OLD', 'writeoff', '2026-08-02 12:00:00+03', 'OIL-001', 'MS-01'),

('DOC-REC-CRM-01', 'receipt', '2026-06-15 14:00:00+03', 'CRM-002', 'MS-01'),
('DOC-REC-SCR-01', 'receipt', '2026-04-10 15:00:00+03', 'SCR-003', 'MS-01'),
('DOC-REC-SCR-02', 'receipt', '2026-08-05 16:00:00+03', 'SCR-003', 'MS-01'),
('DOC-REC-SHP-01', 'receipt', '2026-06-01 10:00:00+03', 'SHP-004', 'MS-01'),
('DOC-REC-LEM-01', 'receipt', '2026-05-01 12:00:00+03', 'OIL-005', 'MS-01'),

('DOC-CSM-OIL-01', 'consume', '2026-06-25 18:00:00+03', 'OIL-001', 'MS-01'),
('DOC-CSM-OIL-02', 'consume', '2026-07-10 18:00:00+03', 'OIL-001', 'MS-01'),
('DOC-CSM-OIL-03', 'consume', '2026-07-28 18:00:00+03', 'OIL-001', 'MS-01'),
('DOC-CSM-OIL-04', 'consume', '2026-08-15 18:00:00+03', 'OIL-001', 'MS-01'),
('DOC-CSM-OIL-05', 'consume', '2026-09-05 18:00:00+03', 'OIL-001', 'MS-01'),

('DOC-CSM-CRM-01', 'consume', '2026-07-01 17:00:00+03', 'CRM-002', 'MS-01'),
('DOC-CSM-CRM-02', 'consume', '2026-07-25 17:00:00+03', 'CRM-002', 'MS-01'),
('DOC-CSM-CRM-03', 'consume', '2026-08-20 17:00:00+03', 'CRM-002', 'MS-01'),
('DOC-CSM-SCR-01', 'consume', '2026-08-10 16:30:00+03', 'SCR-003', 'MS-01'),
('DOC-CSM-SHP-01', 'consume', '2026-08-01 12:00:00+03', 'SHP-004', 'MS-01')
ON CONFLICT (document_no) DO NOTHING;

-- 6. Журнал движений товаров (Movements Ledger)
-- 6.1. Приходные накладные
INSERT INTO movements (document_no, operation_date, sku, location_id, operation_type, quantity, batch_id, note) VALUES
('DOC-REC-OIL-05', '2026-05-10 10:00:00+03', 'OIL-001', 'MS-01', 'receipt', 100.0000, 'BATCH-2026-05', 'Поступление масла май'),
('DOC-REC-OIL-07', '2026-07-20 11:30:00+03', 'OIL-001', 'MS-01', 'receipt', 72.9700, 'BATCH-2026-07', 'Поступление масла июль'),
('DOC-REC-OIL-OLD', '2025-08-01 09:00:00+03', 'OIL-001', 'MS-01', 'receipt', 5.0000, 'BATCH-2025-OLD', 'Старое поступление 2025'),
('DOC-WOF-OIL-OLD', '2026-08-02 12:00:00+03', 'OIL-001', 'MS-01', 'writeoff', 5.0000, 'BATCH-2025-OLD', 'Списание в связи с истечением срока годности'),

('DOC-REC-CRM-01', '2026-06-15 14:00:00+03', 'CRM-002', 'MS-01', 'receipt', 39.0000, 'BATCH-CRM-01', 'Поступление крема'),
('DOC-REC-SCR-01', '2026-04-10 15:00:00+03', 'SCR-003', 'MS-01', 'receipt', 10.0000, 'BATCH-SCR-EXP', 'Поступление скраба детокс'),
('DOC-REC-SCR-02', '2026-08-05 16:00:00+03', 'SCR-003', 'MS-01', 'receipt', 20.0000, 'BATCH-SCR-02', 'Поступление скраба партия 2'),
('DOC-REC-SHP-01', '2026-06-01 10:00:00+03', 'SHP-004', 'MS-01', 'receipt', 50.0000, 'BATCH-SHP-01', 'Поступление простыней'),
('DOC-REC-LEM-01', '2026-05-01 12:00:00+03', 'OIL-005', 'MS-01', 'receipt', 15.0000, 'BATCH-LEM-01', 'Поступление эфирного масла');

-- 6.2. Движения расхода за последние 90 дней (с 2026-06-17 по 2026-09-15)
-- Суммарный расход по OIL-001 за 90 дней = ровно 122.58 л (122.58 / 90 = 1.362 л/день)
-- По BATCH-2026-05 списывается 79.61 л (остается 20.39 л)
-- По BATCH-2026-07 списывается 42.97 л (остается 30.00 л)
INSERT INTO movements (document_no, operation_date, sku, location_id, operation_type, quantity, batch_id, note) VALUES
('DOC-CSM-OIL-01', '2026-06-25 18:00:00+03', 'OIL-001', 'MS-01', 'consume', 25.0000, 'BATCH-2026-05', 'Расход спа-процедуры'),
('DOC-CSM-OIL-02', '2026-07-10 18:00:00+03', 'OIL-001', 'MS-01', 'consume', 25.0000, 'BATCH-2026-05', 'Расход спа-процедуры'),
('DOC-CSM-OIL-03', '2026-07-28 18:00:00+03', 'OIL-001', 'MS-01', 'consume', 29.6100, 'BATCH-2026-05', 'Расход спа-процедуры'),
('DOC-CSM-OIL-04', '2026-08-15 18:00:00+03', 'OIL-001', 'MS-01', 'consume', 20.0000, 'BATCH-2026-07', 'Расход спа-процедуры'),
('DOC-CSM-OIL-05', '2026-09-05 18:00:00+03', 'OIL-001', 'MS-01', 'consume', 22.9700, 'BATCH-2026-07', 'Расход спа-процедуры'),

-- Расход CRM-002: из 39.0 кг прихода израсходовано 36.0 кг за 90 дней (36.0 / 90 = 0.4 кг/день).
-- Текущий остаток 3.0 кг -> запас 3.0 / 0.4 = 7.5 дней при lead_time 10 дней -> ДЕФИЦИТ!
('DOC-CSM-CRM-01', '2026-07-01 17:00:00+03', 'CRM-002', 'MS-01', 'consume', 12.0000, 'BATCH-CRM-01', 'Расход процедур крем'),
('DOC-CSM-CRM-02', '2026-07-25 17:00:00+03', 'CRM-002', 'MS-01', 'consume', 12.0000, 'BATCH-CRM-01', 'Расход процедур крем'),
('DOC-CSM-CRM-03', '2026-08-20 17:00:00+03', 'CRM-002', 'MS-01', 'consume', 12.0000, 'BATCH-CRM-01', 'Расход процедур крем'),

-- Расход SCR-003: израсходовано 5.0 кг из BATCH-SCR-EXP, остаток партии 5.0 кг истекает через 15 дней
('DOC-CSM-SCR-01', '2026-08-10 16:30:00+03', 'SCR-003', 'MS-01', 'consume', 5.0000, 'BATCH-SCR-EXP', 'Расход процедур скраб'),

-- Расход SHP-004: израсходовано 15 рулонов, остаток 35 рулонов
('DOC-CSM-SHP-01', '2026-08-01 12:00:00+03', 'SHP-004', 'MS-01', 'consume', 15.0000, 'BATCH-SHP-01', 'Расход простыней');

-- 7. Открытые заказы в пути
INSERT INTO open_orders (id, sku, location_id, supplier_id, quantity, order_date, expected_delivery_date, status) VALUES
('ORD-2026-09-12', 'OIL-001', 'MS-01', 'SUP-01', 20.0000, '2026-09-12', '2026-09-25', 'in_transit')
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DELETE FROM open_orders;
DELETE FROM movements;
DELETE FROM processed_documents;
DELETE FROM batches;
DELETE FROM products;
DELETE FROM suppliers;
DELETE FROM locations;
