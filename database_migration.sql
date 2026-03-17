-- 1. Tambahkan Column baru ke tabel yang sudah ada
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS gateway_order_id VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_transactions_gateway_order_id ON transactions(gateway_order_id);

-- 2. Buat Tabel Baru untuk Ledger (Log Keuangan)
CREATE TABLE IF NOT EXISTS ledgers (
    id SERIAL PRIMARY KEY,
    project_id INT NOT NULL,
    transaction_id INT NOT NULL,
    amount DECIMAL(15,2) NOT NULL,
    type VARCHAR(10) NOT NULL, -- 'credit' atau 'debit'
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 3. Buat Tabel Baru untuk Audit Log (Log Perubahan Saldo)
CREATE TABLE IF NOT EXISTS audit_logs (
    id SERIAL PRIMARY KEY,
    project_id INT NOT NULL,
    transaction_id INT NOT NULL,
    before_balance DECIMAL(15,2) NOT NULL,
    after_balance DECIMAL(15,2) NOT NULL,
    amount DECIMAL(15,2) NOT NULL,
    type VARCHAR(10) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 4. Tambahkan Index untuk mempercepat pencarian
CREATE INDEX IF NOT EXISTS idx_ledgers_project ON ledgers(project_id);
CREATE INDEX IF NOT EXISTS idx_audit_project ON audit_logs(project_id);
CREATE INDEX IF NOT EXISTS idx_transactions_order_id ON transactions(order_id);
CREATE INDEX IF NOT EXISTS idx_transactions_reference ON transactions(reference);

-- 5. Tambahkan Unique Constraint untuk mencegah duplikasi order_id per project
-- dan memastikan reference unik secara sistem
ALTER TABLE transactions ADD CONSTRAINT unique_project_order UNIQUE (project_id, order_id);
ALTER TABLE transactions ADD CONSTRAINT unique_reference UNIQUE (reference);

-- 6. Tabel Relasi Project dan Metode Pembayaran (Metode yang diaktifkan merchant)
CREATE TABLE IF NOT EXISTS project_payment_methods (
    id SERIAL PRIMARY KEY,
    project_id INT NOT NULL,
    payment_method_id INT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project_id, payment_method_id)
);

CREATE INDEX IF NOT EXISTS idx_project_pm_project ON project_payment_methods(project_id);
