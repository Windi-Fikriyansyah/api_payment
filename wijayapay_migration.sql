-- Update Gateway Codes for WijayaPay
-- Based on: https://docs.wijayapay.com/

UPDATE payment_methods SET gateway_code = 'QRIS' WHERE code = 'qris';
UPDATE payment_methods SET gateway_code = 'BRIVA' WHERE code = 'bri_va';
UPDATE payment_methods SET gateway_code = 'BNIVA' WHERE code = 'bni_va';
UPDATE payment_methods SET gateway_code = 'MANDIRIVA' WHERE code = 'mandiri_va';
UPDATE payment_methods SET gateway_code = 'BCAVA' WHERE code = 'bca_va';
UPDATE payment_methods SET gateway_code = 'PERMATAVA' WHERE code = 'permata_va';
UPDATE payment_methods SET gateway_code = 'MAYBANKVA' WHERE code = 'maybank_va';
UPDATE payment_methods SET gateway_code = 'CIMBVA' WHERE code = 'cimb_niaga_va';
UPDATE payment_methods SET gateway_code = 'DANAMONVA' WHERE code = 'danamon_va';
UPDATE payment_methods SET gateway_code = 'BANKNEOVA' WHERE code = 'bnc_va';
UPDATE payment_methods SET gateway_code = 'ALFAMART' WHERE code = 'alfamart';
UPDATE payment_methods SET gateway_code = 'INDOMARET' WHERE code = 'indomaret';
