-- Tabel Metode Pembayaran
CREATE TABLE IF NOT EXISTS payment_methods (
    id SERIAL PRIMARY KEY,
    code VARCHAR(50) UNIQUE NOT NULL, -- e.g. 'qris', 'bca_va'
    gateway_code VARCHAR(20) NOT NULL, -- e.g. 'QRIS', 'BCAVA'
    name VARCHAR(100) NOT NULL,
    image_url TEXT,
    fee_flat DECIMAL(15,2) DEFAULT 0,
    fee_percent DECIMAL(5,4) DEFAULT 0, -- e.g. 0.0070 for 0.7%
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Seeder Data (Sesuai tarif iPaymu)
INSERT INTO payment_methods (code, gateway_code, name, fee_flat, fee_percent, image_url) VALUES
('qris', 'qris', 'QRIS', 0, 0.0070, 'https://app.linkbayar.my.id/images/qris.png'),
('bri_va', 'bri', 'BRI Virtual Account', 3500, 0, 'https://app.linkbayar.my.id/images/bri.png'),
('bni_va', 'bni', 'BNI Virtual Account', 3500, 0, 'https://app.linkbayar.my.id/images/bni.png'),
('mandiri_va', 'mandiri', 'Mandiri Virtual Account', 3500, 0, 'https://app.linkbayar.my.id/images/mandiri.png'),
('bca_va', 'bca', 'BCA Virtual Account', 5500, 0, 'https://app.linkbayar.my.id/images/bca.png'),
('permata_va', 'permata', 'Permata Virtual Account', 3500, 0, 'https://app.linkbayar.my.id/images/permata.png'),
('maybank_va', 'maybank', 'Maybank Virtual Account', 3500, 0, 'https://app.linkbayar.my.id/images/maybank.png'),
('cimb_niaga_va', 'cimb', 'CIMB Niaga Virtual Account', 3500, 0, 'https://app.linkbayar.my.id/images/cimb.png'),
('danamon_va', 'danamon', 'Danamon Virtual Account', 3500, 0, 'https://app.linkbayar.my.id/images/danamon.png'),
('bnc_va', 'bnc', 'BNC Virtual Account', 3500, 0, 'https://app.linkbayar.my.id/images/bnc.png'),
('alfamart', 'alfamart', 'Alfamart', 5000, 0, 'https://app.linkbayar.my.id/images/alfamart.png'),
('indomaret', 'indomaret', 'Indomaret', 5000, 0, 'https://app.linkbayar.my.id/images/indomaret.png')
ON CONFLICT (code) DO UPDATE SET 
    fee_flat = EXCLUDED.fee_flat, 
    fee_percent = EXCLUDED.fee_percent,
    gateway_code = EXCLUDED.gateway_code,
    name = EXCLUDED.name;
