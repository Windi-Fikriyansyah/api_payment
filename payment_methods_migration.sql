-- Tabel Metode Pembayaran
CREATE TABLE IF NOT EXISTS payment_methods (
    id SERIAL PRIMARY KEY,
    code VARCHAR(50) UNIQUE NOT NULL, -- e.g. 'qris', 'bca_va'
    duitku_code VARCHAR(10) NOT NULL, -- e.g. 'SP', 'BC'
    name VARCHAR(100) NOT NULL,
    image_url TEXT,
    fee_flat DECIMAL(15,2) DEFAULT 0,
    fee_percent DECIMAL(5,4) DEFAULT 0, -- e.g. 0.0070 for 0.7%
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Seeder Data (Sesuai tarif saat ini)
INSERT INTO payment_methods (code, duitku_code, name, fee_flat, fee_percent, image_url) VALUES
('qris', 'SP', 'QRIS', 310, 0.0070, 'https://app.winlink.com/images/qris.png'),
('bri_va', 'BR', 'BRI Virtual Account', 3500, 0, 'https://app.winlink.com/images/bri.png'),
('bni_va', 'I1', 'BNI Virtual Account', 3500, 0, 'https://app.winlink.com/images/bni.png'),
('mandiri_va', 'M2', 'Mandiri Virtual Account', 4500, 0, 'https://app.winlink.com/images/mandiri.png'),
('bca_va', 'BC', 'BCA Virtual Account', 5500, 0, 'https://app.winlink.com/images/bca.png'),
('atm_bersama_va', 'A1', 'ATM Bersama', 3500, 0, 'https://app.winlink.com/images/atm_bersama.png'),
('bnc_va', 'BN', 'BNC Virtual Account', 3500, 0, 'https://app.winlink.com/images/bnc.png'),
('cimb_niaga_va', 'B1', 'CIMB Niaga Virtual Account', 3500, 0, 'https://app.winlink.com/images/cimb.png'),
('maybank_va', 'VA', 'Maybank Virtual Account', 3500, 0, 'https://app.winlink.com/images/maybank.png'),
('permata_va', 'BT', 'Permata Virtual Account', 3500, 0, 'https://app.winlink.com/images/permata.png'),
('artha_graha_va', 'AG', 'Artha Graha Virtual Account', 2000, 0, 'https://app.winlink.com/images/artha_graha.png'),
('sampoerna_va', 'SA', 'Sampoerna Virtual Account', 2000, 0, 'https://app.winlink.com/images/sampoerna.png')
ON CONFLICT (code) DO UPDATE SET 
    fee_flat = EXCLUDED.fee_flat, 
    fee_percent = EXCLUDED.fee_percent,
    duitku_code = EXCLUDED.duitku_code,
    name = EXCLUDED.name;
